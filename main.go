package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"
)

var (
	zapLogger, _ = zap.NewProduction()
	log          = zapLogger.Sugar()

	//apiServer = os.Getenv("KUBERNETES_APISERVER")
	apiServer      = "https://kubernetes.default.svc"
	longhornServer = "http://longhorn-frontend.core-services.svc"

	minioRestoreJobName = "minio-restore"
)

// type CronJob struct {
// 	Type          string `json:"type"`
// 	Initialized   bool   `json:"initialized"`
// 	KeysThreshold int    `json:"t"`
// }

func readFile(path string) string {
	fileBytes, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("Error reading file: %s", err)
		return ""
	}

	return string(fileBytes)
}

func sendRequest(method string, url string, token string, body io.Reader) ([]byte, error) {
	// Prepare the request
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		log.Fatalf("Error preparing request: %s", err)
	}

	// Headers
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	// Send the request
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
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

func triggerCronJob(name string, namespace string, token string) error {
	// Get the Cronjob info
	cronJob, err := sendRequest("GET", fmt.Sprintf("%s/apis/batch/v1/namespaces/%s/cronjobs/%s", apiServer, namespace, name), token, nil)
	if err != nil {
		log.Errorf("Error retreiving cronjob info: %s", err)
	}

	log.Infof("Cronjob: %s", string(cronJob))

	return nil
}

func main() {
	// Variables
	namespace := readFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	token := readFile("/var/run/secrets/kubernetes.io/serviceaccount/token")

	// Check if backup volumes exist
	log.Info("Checking for backup volumes:")

	backupVolumes, err := sendRequest("GET", fmt.Sprintf("%s/v1/backupvolumes", longhornServer), "", nil)
	if err != nil {
		log.Errorf("Error retreiving backup volume info: %s", err)
	}

	// [Debug] Do something
	log.Infof("Backup volume info: %s", string(backupVolumes))

	// Trigger cronjob to restore the backup
	err = triggerCronJob(minioRestoreJobName, namespace, token)
	if err != nil {
		log.Errorf("Error running the backup restore operation: %s", err)
	}

	// [Debug] to keep alive for attaching
	for {
		time.Sleep(time.Duration(60) * time.Second)
	}
}
