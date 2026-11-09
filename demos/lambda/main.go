// Demo 2 (слайди 33–36): Go Lambda для Floci, runtime provided.al2023.
// Окремий Go-модуль, не пов'язаний з LibraTrack.
package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-lambda-go/lambda"
)

type Event struct {
	Name string `json:"name"`
}

type Response struct {
	StatusCode int    `json:"statusCode"`
	Body       string `json:"body"`
}

func handler(ctx context.Context, e Event) (Response, error) {
	name := e.Name
	if name == "" {
		name = "world"
	}
	return Response{StatusCode: 200, Body: fmt.Sprintf("hello, %s from floci", name)}, nil
}

func main() {
	lambda.Start(handler)
}
