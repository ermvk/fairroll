// CURRENCY SERVICE
// gRPC service for currency convert

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	"fairroll/pkg/config"
	"fairroll/pkg/logger"
	pb "fairroll/services/currency/api"
	"fairroll/services/currency/internal/consumer"
	"fairroll/services/currency/internal/service"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := godotenv.Load(); err != nil {
		log.Println("No .env.example found")
	}

	cfg, err := config.LoadConfig(ctx)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	loggerCfg := logger.Config{
		Loglevel: cfg.Logger.Level,
		Format:   cfg.Logger.Format,
		OutPut:   "stdout",
	}

	if err := logger.Init(loggerCfg); err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}

	defer logger.Sync()

	logger.Info(ctx, "Starting Currency Service (gRPC)",
		"name", cfg.Service.Name,
		"version", cfg.Service.Version,
		"environment", cfg.Environment,
	)

	currencyService := service.NewCurrencyService()

	grpcPortStr := os.Getenv("GRPC_PORT")
	if grpcPortStr == "" {
		grpcPortStr = "50051"
	}

	grpcPort := fmt.Sprintf(":%s", grpcPortStr)
	lis, err := net.Listen("tcp", grpcPort)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", grpcPort, err)
	}

	grpcServer := grpc.NewServer()

	pb.RegisterCurrencyServiceServer(grpcServer, newCurrencyServiceServer(currencyService))

	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("fairroll.currency.CurrencyService", grpc_health_v1.HealthCheckResponse_SERVING)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","service":"currency","timestamp":"%s"}`,
			time.Now().Format(time.RFC3339))
	})

	httpAddr := fmt.Sprintf(":%d", cfg.Service.Port) // 8085 из docker-compose
	httpServer := &http.Server{Addr: httpAddr, Handler: mux}

	go func() {
		logger.Info(ctx, "HTTP health server started", "address", httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error(ctx, "HTTP health server error", err)
		}
	}()
	go func() {
		logger.Info(ctx, "gRPC server listening", "address", grpcPort)
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error(ctx, "gRPC server error", err)
		}
	}()

	kafkaClient, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Kafka.Brokers...),
		kgo.ConsumeTopics("conversion.requested"),
		kgo.ConsumerGroup("currency-service"),
	)
	if err != nil {
		log.Fatalf("Failed to create kafka client: %v", err)
	}
	defer kafkaClient.Close()

	producer, err := kgo.NewClient(kgo.SeedBrokers(cfg.Kafka.Brokers...))
	if err != nil {
		log.Fatalf("Failed to create kafka producer: %v", err)
	}
	defer producer.Close()

	conversionConsumer := consumer.NewConversionConsumer(kafkaClient, producer, currencyService)
	consumerCtx, consumerCancel := context.WithCancel(context.Background())
	defer consumerCancel()
	go conversionConsumer.Run(consumerCtx)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	logger.Info(ctx, "Shutdown signal received", "signal", sig.String())

	grpcServer.GracefulStop()
	logger.Info(ctx, "Currency Service shutdown complete")
}

type currencyServiceServer struct {
	pb.UnimplementedCurrencyServiceServer
	service *service.CurrencyService
}

func newCurrencyServiceServer(svc *service.CurrencyService) *currencyServiceServer {
	return &currencyServiceServer{
		service: svc,
	}
}

func (s *currencyServiceServer) Convert(ctx context.Context, req *pb.ConvertRequest) (*pb.ConvertResponse, error) {
	// Вызываем сервис
	convertedAmount, err := s.service.ConvertViaGRPC(req.Amount, req.FromCurrency, req.ToCurrency)
	if err != nil {
		return &pb.ConvertResponse{
			Error: err.Error(),
		}, nil
	}

	rate, _ := s.service.GetExchangeRate(req.FromCurrency, req.ToCurrency)

	return &pb.ConvertResponse{
		OriginalAmount:  req.Amount,
		ConvertedAmount: convertedAmount,
		Rate:            rate.String(),
	}, nil
}

func (s *currencyServiceServer) GetRate(ctx context.Context, req *pb.GetRateRequest) (*pb.GetRateResponse, error) {
	rate, err := s.service.GetExchangeRate(req.FromCurrency, req.ToCurrency)
	if err != nil {
		return &pb.GetRateResponse{
			Error: err.Error(),
		}, nil
	}

	return &pb.GetRateResponse{
		Rate: rate.String(),
	}, nil
}

func (s *currencyServiceServer) GetRates(ctx context.Context, req *pb.GetRatesRequest) (*pb.GetRatesResponse, error) {
	rates, err := s.service.GetRates(req.BaseCurrency)
	if err != nil {
		return &pb.GetRatesResponse{
			Error: err.Error(),
		}, nil
	}

	ratesMap := make(map[string]string)
	for currency, rate := range rates {
		ratesMap[currency] = rate.String()
	}

	return &pb.GetRatesResponse{
		Rates: ratesMap,
	}, nil
}

func (s *currencyServiceServer) GetSupportedCurrencies(ctx context.Context, req *pb.Empty) (*pb.SupportedCurrenciesResponse, error) {
	currencies := s.service.GetSupportedCurrencies()

	return &pb.SupportedCurrenciesResponse{
		Currencies: currencies,
	}, nil
}
