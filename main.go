package main

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

var (
	zapLogger, _ = zap.NewProduction()
	log          = zapLogger.Sugar()

	//apiServer = os.Getenv("KUBERNETES_APISERVER")
	//apiServer      = "https://kubernetes.default.svc"
	longhornServer = "http://longhorn-frontend.core-services.svc"
)

func sendRequest(method string, url string, body io.Reader) ([]byte, error) {
	// Send the request
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		log.Fatalf("Error preparing request: %s", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("Error performing request: %s", err)
	}

	// Return response body as string
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Error reading response body: %s", err)
	}

	defer resp.Body.Close()
	return out, nil
}

func main() {
	// Check if backup volumes exist
	log.Info("Checking for backup volumes:")

	backupVolumes, err := sendRequest("GET", fmt.Sprintf("%s/v1/backupvolumes", longhornServer), nil)
	if err != nil {
		log.Errorf("Error retreiving backup volume info: %s", err)
	}

	log.Infof("Backup volume info:\n%s\n", string(backupVolumes))

	// [Debug] to keep alive for attaching
	for {
		time.Sleep(time.Duration(60) * time.Second)
	}
}
