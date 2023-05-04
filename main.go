package main

import (
	"go.uber.org/zap"
)

var (
	zapLogger, _ = zap.NewProduction()
	log          = zapLogger.Sugar()

	//apiServer = os.Getenv("KUBERNETES_APISERVER")
	//apiServer = "https://kubernetes.default.svc"
)

func main() {
	// Print a success message
	log.Info("Testing.")
}
