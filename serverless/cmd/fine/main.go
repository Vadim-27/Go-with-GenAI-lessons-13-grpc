// Fine-функція LibraTrack: приймає JSON про повернення позики через API Gateway
// та розраховує штраф за прострочення. Домашнє завдання уроку 11 (пункт 1).
package main

import (
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
)

// Логер на рівні пакета — створюється один раз у init-фазі (слайд 23).
var logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

// main() лише передає керування SDK — уся логіка в handler (слайд 18).
func main() {
	lambda.Start(handler)
}
