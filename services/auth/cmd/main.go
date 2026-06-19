package main

import (
	"context"
	"time"
)

func main() {

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)

}
