package service

import (
	"github.com/shopspring/decimal"
)

type CurrencyService struct {
	rates map[string]decimal.Decimal
}

func NewCurrencyService() *CurrencyService {
	cs := &CurrencyService{
		rates: make(map[string]decimal.Decimal),
	}
	cs.initializeRates()
	return cs
}

func (s *CurrencyService) initializeRates() {
	// USD to other currencies
	s.rates["USD->USD"] = decimal.NewFromInt(1)
	s.rates["USD->EUR"], _ = decimal.NewFromString("0.92")
	s.rates["USD->GBP"], _ = decimal.NewFromString("0.79")
	s.rates["USD->JPY"], _ = decimal.NewFromString("150.50")
	s.rates["USD->CAD"], _ = decimal.NewFromString("1.36")
	s.rates["USD->CHF"], _ = decimal.NewFromString("0.88")

	// EUR conversions
	s.rates["EUR->USD"], _ = decimal.NewFromString("1.09")
	s.rates["EUR->EUR"] = decimal.NewFromInt(1)
	s.rates["EUR->GBP"], _ = decimal.NewFromString("0.86")
	s.rates["EUR->JPY"], _ = decimal.NewFromString("164.67")
	s.rates["EUR->CAD"], _ = decimal.NewFromString("1.48")
	s.rates["EUR->CHF"], _ = decimal.NewFromString("0.96")

	// GBP conversions
	s.rates["GBP->USD"], _ = decimal.NewFromString("1.27")
	s.rates["GBP->EUR"], _ = decimal.NewFromString("1.16")
	s.rates["GBP->GBP"] = decimal.NewFromInt(1)
	s.rates["GBP->JPY"], _ = decimal.NewFromString("190.51")
	s.rates["GBP->CAD"], _ = decimal.NewFromString("1.72")
	s.rates["GBP->CHF"], _ = decimal.NewFromString("1.11")

	s.rates["JPY->USD"], _ = decimal.NewFromString("0.0066")
	s.rates["JPY->EUR"], _ = decimal.NewFromString("0.0061")
	s.rates["JPY->GBP"], _ = decimal.NewFromString("0.0052")
	s.rates["JPY->JPY"] = decimal.NewFromInt(1)
	s.rates["JPY->CAD"], _ = decimal.NewFromString("0.0090")
	s.rates["JPY->CHF"], _ = decimal.NewFromString("0.0058")

	s.rates["CAD->USD"], _ = decimal.NewFromString("0.74")
	s.rates["CAD->EUR"], _ = decimal.NewFromString("0.68")
	s.rates["CAD->GBP"], _ = decimal.NewFromString("0.58")
	s.rates["CAD->JPY"], _ = decimal.NewFromString("110.44")
	s.rates["CAD->CAD"] = decimal.NewFromInt(1)
	s.rates["CAD->CHF"], _ = decimal.NewFromString("0.65")

	s.rates["CHF->USD"], _ = decimal.NewFromString("1.14")
	s.rates["CHF->EUR"], _ = decimal.NewFromString("1.04")
	s.rates["CHF->GBP"], _ = decimal.NewFromString("0.90")
	s.rates["CHF->JPY"], _ = decimal.NewFromString("170.46")
	s.rates["CHF->CAD"], _ = decimal.NewFromString("1.55")
	s.rates["CHF->CHF"] = decimal.NewFromInt(1)
}
func (s *CurrencyService) Convert(fromCurrency, toCurrency string, amount decimal.Decimal) (decimal.Decimal, error) {
	if fromCurrency == toCurrency {
		return amount, nil
	}

	key := fromCurrency + "->" + toCurrency
	rate, exists := s.rates[key]
	if !exists {
		return decimal.Zero, ErrCurrencyNotSupported
	}

	return amount.Mul(rate), nil
}

func (s *CurrencyService) GetExchangeRate(fromCurrency, toCurrency string) (decimal.Decimal, error) {
	if fromCurrency == toCurrency {
		return decimal.NewFromInt(1), nil
	}

	key := fromCurrency + "->" + toCurrency
	rate, exists := s.rates[key]
	if !exists {
		return decimal.Zero, ErrCurrencyNotSupported
	}

	return rate, nil
}

func (s *CurrencyService) GetRates(baseCurrency string) (map[string]decimal.Decimal, error) {
	result := make(map[string]decimal.Decimal)

	for key, rate := range s.rates {
		if len(key) >= len(baseCurrency)+2 && key[:len(baseCurrency)] == baseCurrency && key[len(baseCurrency):len(baseCurrency)+2] == "->" {
			targetCurrency := key[len(baseCurrency)+2:]
			result[targetCurrency] = rate
		}
	}

	if len(result) == 0 {
		return nil, ErrCurrencyNotSupported
	}

	return result, nil
}

func (s *CurrencyService) GetSupportedCurrencies() []string {
	supportedSet := make(map[string]bool)

	for key := range s.rates {
		fromCurrency := key[:3]
		toCurrency := key[5:8]

		supportedSet[fromCurrency] = true
		supportedSet[toCurrency] = true
	}

	var currencies []string
	for currency := range supportedSet {
		currencies = append(currencies, currency)
	}

	return currencies
}

func (s *CurrencyService) ConvertViaGRPC(amountStr, fromCurrency, toCurrency string) (string, error) {
	amount, err := decimal.NewFromString(amountStr)
	if err != nil {
		return "", err
	}

	converted, err := s.Convert(fromCurrency, toCurrency, amount)
	if err != nil {
		return "", err
	}

	return converted.String(), nil
}
