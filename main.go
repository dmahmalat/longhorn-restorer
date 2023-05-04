package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tidwall/gjson"
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

func randomAlphaNumeric(length int) string {
	charSet := "abcdefghijklmnopqrstuvwxyz0123456789"
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	b := make([]byte, length)
	for i := range b {
		b[i] = charSet[rng.Intn(len(charSet)-1)]
	}

	return string(b)
}

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

	// Fetch variables
	cronJobName := gjson.Get(string(cronJob), "metadata.name").String()
	cronJobUid := gjson.Get(string(cronJob), "metadata.uid").String()
	jobSpec := gjson.Get(string(cronJob), "spec.jobTemplate.spec.template.spec").Raw

	// Generate the Job name
	jobName := fmt.Sprintf("%s-manual-%s",
		cronJobName,
		randomAlphaNumeric(5),
	)

	// Trim max Kubernetes allowed name length
	if len(jobName) > 63 {
		jobName = jobName[:63]
	}

	// Build the job JSON
	jobJson := `{
		"apiVersion": "batch/v1",
		"kind": "Job",
		"metadata": {
			"name": "` + jobName + `",
			"namespace": "` + namespace + `",
			"annotations": {
				"cronjob.kubernetes.io/instantiate": "manual"
			},
			"ownerReferences": [{
				"apiVersion": "batch/v1",
				"kind": "CronJob",
				"name": "` + cronJobName + `",
				"uid": "` + cronJobUid + `",
				"blockOwnerDeletion": true,
				"controller": true
			}]
		},
		"spec": {
			"template": {
				"spec": ` + jobSpec + `
			}
		}
	}`

	// Start manual job
	_, err = sendRequest("POST", fmt.Sprintf("%s/apis/batch/v1/namespaces/%s/jobs", apiServer, namespace), token, strings.NewReader(jobJson))
	if err != nil {
		log.Errorf("Error manually creating job %s: %s", jobName, err)
	}

	log.Infof("Created manual job: %s", jobName)
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

	// Check if backup volumes exist
	// [DEBUG] Add named volumes
	volumeData := gjson.Get(string(backupVolumes), "data").Raw
	if volumeData == "[]" {
		// Trigger cronjob to restore the backup
		log.Info("No volumes found. Restoring.")
		err = triggerCronJob(minioRestoreJobName, namespace, token)
		if err != nil {
			log.Errorf("Error running the backup restore operation: %s", err)
		}
	} else {
		log.Info("Volumes already restored. Nothing to do.")
	}

	// [Debug] to keep alive for attaching
	for {
		time.Sleep(time.Duration(60) * time.Second)
	}
}
