package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/joho/godotenv"

	"sharedmodule"

	"huaweicloud.com/go-runtime/events/smn"
	fgcontext "huaweicloud.com/go-runtime/go-api/context"
	"huaweicloud.com/go-runtime/pkg/runtime"
)

//go:embed kubernetes-templates/*
var kubernetesTemplates embed.FS

type Config struct {
	HostingBuilderImage     string
	ImagePullPolicy         string
	AccessKey               string
	SecretKey               string
	ProjectName             string
	DependencyPath          string
	PrintOutFile            bool
	K8sNamespace            string
	CciIamAuthenticatorPath string
	KubectlPath             string
	LxdHost                 string
	AnsibleUser             string
	AnsiblePassword         string
	DbRootHost              string
	DbRootUser              string
	DbRootPassword          string
	TopicUrn                string
}

var appConfig Config

func loadConfig() {
	if err := godotenv.Load(); err != nil {
		slog.Warn("failed to load .env file", slog.String("error", err.Error()))
	}

	appConfig.HostingBuilderImage = sharedmodule.GetEnv("HOSTING_BUILDER_IMAGE", "swr.ap-southeast-4.myhuaweicloud.com/demo-huawei/hostingbuilder:latest")
	appConfig.ImagePullPolicy = sharedmodule.GetEnv("IMAGE_PULL_POLICY", "IfNotPresent")
	appConfig.AccessKey = sharedmodule.GetEnv("ACCESS_KEY", "")
	appConfig.SecretKey = sharedmodule.GetEnv("SECRET_KEY", "")
	appConfig.ProjectName = sharedmodule.GetEnv("PROJECT_NAME", "ap-southeast-4")
	appConfig.DependencyPath = sharedmodule.GetEnv("DEPENDENCY_PATH", "./code")
	appConfig.PrintOutFile, _ = strconv.ParseBool(sharedmodule.GetEnv("PRINT_OUT_FILE", "false"))
	appConfig.K8sNamespace = sharedmodule.GetEnv("K8S_NAMESPACE", "default")
	appConfig.CciIamAuthenticatorPath = sharedmodule.GetEnv("CCI_IAM_AUTHENTICATOR_PATH", "")
	appConfig.KubectlPath = sharedmodule.GetEnv("KUBECTL_PATH", "")
	appConfig.LxdHost = sharedmodule.GetEnv("LXD_HOST", "")
	appConfig.AnsibleUser = sharedmodule.GetEnv("ANSIBLE_USER", "root")
	appConfig.AnsiblePassword = sharedmodule.GetEnv("ANSIBLE_PASSWORD", "")
	appConfig.DbRootHost = sharedmodule.GetEnv("DB_ROOT_HOST", "localhost")
	appConfig.DbRootUser = sharedmodule.GetEnv("DB_ROOT_USER", "root")
	appConfig.DbRootPassword = sharedmodule.GetEnv("DB_ROOT_PASSWORD", "")
	appConfig.TopicUrn = sharedmodule.GetEnv("TOPIC_URN", "")
}

func getCCIToken() (string, error) {

	cciIamAuthenticatorPath := appConfig.DependencyPath + "/cci-iam-authenticator"
	if appConfig.CciIamAuthenticatorPath != "" {
		cciIamAuthenticatorPath = appConfig.CciIamAuthenticatorPath
	}

	cmd := exec.Command(cciIamAuthenticatorPath,
		"token",
		"--iam-endpoint=https://iam.myhuaweicloud.com",
		"--insecure-skip-tls-verify=true",
		"--cache=false",
		"--token-only=true",
		"--project-name="+appConfig.ProjectName,
		"--ak="+appConfig.AccessKey,
		"--sk="+appConfig.SecretKey,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get token: %w\nOutput: %s", err, string(out))
	}

	token := strings.TrimSpace(string(out))
	return token, nil
}

func applyK8s(token string, yamlContent []byte) error {

	kubectlPath := appConfig.DependencyPath + "/kubectl"
	if appConfig.KubectlPath != "" {
		kubectlPath = appConfig.KubectlPath
	}

	cmd := exec.Command(kubectlPath,
		"--token="+token,
		"--server=https://cci."+appConfig.ProjectName+".myhuaweicloud.com",
		"--insecure-skip-tls-verify=true",
		"--v=0",
		"apply", "-f", "-")

	// Set up stdin to pass the YAML content
	cmd.Stdin = bytes.NewReader(yamlContent)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl error: %v\nOutput: %s", err, string(out))
	}

	fmt.Println(string(out))
	return nil
}

type JobStatus struct {
	Succeeded int    `json:"succeeded"`
	Failed    int    `json:"failed"`
	Active    int    `json:"active"`
	Phase     string `json:"phase,omitempty"`
}

func getJobStatus(token string, jobName string) (*JobStatus, error) {
	kubectlPath := appConfig.DependencyPath + "/kubectl"
	if appConfig.KubectlPath != "" {
		kubectlPath = appConfig.KubectlPath
	}

	cmd := exec.Command(kubectlPath,
		"--token="+token,
		"--server=https://cci."+appConfig.ProjectName+".myhuaweicloud.com",
		"--insecure-skip-tls-verify=true",
		"--v=0",
		"get", "job/"+jobName,
		"--namespace="+appConfig.K8sNamespace,
		"-o", "jsonpath={.status.succeeded},{.status.failed},{.status.active}",
	)

	// Capture only stdout to avoid error message pollution
	out, err := cmd.Output()
	if err != nil {
		// Check if job doesn't exist
		if strings.Contains(string(out), "NotFound") || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("job %s not found or was deleted", jobName)
		}
		return nil, fmt.Errorf("failed to get job status: %v\nOutput: %s", err, string(out))
	}

	// Clean the output by removing kubectl error lines and extracting the actual data
	rawOutput := strings.TrimSpace(string(out))

	// Extract the actual data line (should be the last line that contains comma-separated values)
	lines := strings.Split(rawOutput, "\n")
	var dataLine string
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		// Look for a line that looks like our data format (contains commas and numbers)
		if strings.Contains(line, ",") && !strings.HasPrefix(line, "E") && !strings.Contains(line, "memcache") {
			dataLine = line
			break
		}
	}

	if dataLine == "" {
		dataLine = rawOutput // Fallback to original if no clean line found
	}

	// Parse the comma-separated values
	values := strings.Split(dataLine, ",")
	status := &JobStatus{}

	// Handle empty or incomplete responses
	for i := len(values); i < 3; i++ {
		values = append(values, "")
	}

	if len(values) >= 1 && values[0] != "" && values[0] != "<none>" {
		if succeeded, err := strconv.Atoi(strings.TrimSpace(values[0])); err == nil {
			status.Succeeded = succeeded
		}
	}
	if len(values) >= 2 && values[1] != "" && values[1] != "<none>" {
		if failed, err := strconv.Atoi(strings.TrimSpace(values[1])); err == nil {
			status.Failed = failed
		}
	}
	if len(values) >= 3 && values[2] != "" && values[2] != "<none>" {
		if active, err := strconv.Atoi(strings.TrimSpace(values[2])); err == nil {
			status.Active = active
		}
	}

	return status, nil
}

func isJobComplete(token string, jobName string) (bool, bool, error) {
	kubectlPath := appConfig.DependencyPath + "/kubectl"
	if appConfig.KubectlPath != "" {
		kubectlPath = appConfig.KubectlPath
	}

	// Check job conditions using a more reliable approach
	cmd := exec.Command(kubectlPath,
		"--token="+token,
		"--server=https://cci."+appConfig.ProjectName+".myhuaweicloud.com",
		"--insecure-skip-tls-verify=true",
		"--v=0",
		"get", "job/"+jobName,
		"--namespace="+appConfig.K8sNamespace,
		"-o", "jsonpath={.status.conditions[?(@.type==\"Complete\")].status},{.status.conditions[?(@.type==\"Failed\")].status}",
	)

	// Capture only stdout to avoid error message pollution
	out, err := cmd.Output()
	if err != nil {
		// Check if job doesn't exist
		if strings.Contains(string(out), "NotFound") || strings.Contains(err.Error(), "not found") {
			return false, false, fmt.Errorf("job %s not found or was deleted", jobName)
		}
		return false, false, fmt.Errorf("failed to get job conditions: %v\nOutput: %s", err, string(out))
	}

	rawConditions := strings.TrimSpace(string(out))

	// Clean the conditions output by extracting the actual data line
	lines := strings.Split(rawConditions, "\n")
	var conditionsLine string
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		// Look for a line that contains our condition format (True, False, or comma)
		if (strings.Contains(line, "True") || strings.Contains(line, "False") || strings.Contains(line, ",")) &&
			!strings.HasPrefix(line, "E") && !strings.Contains(line, "memcache") {
			conditionsLine = line
			break
		}
	}

	if conditionsLine == "" {
		conditionsLine = rawConditions // Fallback to original if no clean line found
	}

	parts := strings.Split(conditionsLine, ",")

	// Check Complete condition
	complete := len(parts) > 0 && strings.TrimSpace(parts[0]) == "True"

	// Check Failed condition
	failed := len(parts) > 1 && strings.TrimSpace(parts[1]) == "True"

	return complete, failed, nil
}

func waitK8sJobCompletion(token string, jobName string) error {
	const (
		maxWaitTime     = 10 * time.Minute // Maximum wait time
		pollingInterval = 5 * time.Second  // Check every 5 seconds
	)

	kubectlPath := appConfig.DependencyPath + "/kubectl"
	if appConfig.KubectlPath != "" {
		kubectlPath = appConfig.KubectlPath
	}

	startTime := time.Now()
	fmt.Printf("Waiting for job %s to complete...\n", jobName)

	for {
		// Check if we've exceeded maximum wait time
		if time.Since(startTime) > maxWaitTime {
			return fmt.Errorf("job %s timed out after %v", jobName, maxWaitTime)
		}

		status, err := getJobStatus(token, jobName)
		if err != nil {
			return err
		}

		// First try the more reliable condition-based approach
		complete, failed, condErr := isJobComplete(token, jobName)
		if condErr == nil {
			if complete {
				fmt.Printf("Job %s completed successfully (via conditions)\n", jobName)
				return nil
			}
			if failed {
				// Get more detailed error information
				cmd := exec.Command(kubectlPath,
					"--token="+token,
					"--server=https://cci."+appConfig.ProjectName+".myhuaweicloud.com",
					"--insecure-skip-tls-verify=true",
					"--v=0",
					"describe", "job/"+jobName,
					"--namespace="+appConfig.K8sNamespace,
				)
				out, _ := cmd.CombinedOutput()
				return fmt.Errorf("job %s failed (via conditions)\nJob details:\n%s", jobName, string(out))
			}
		}

		// Fallback to the original status count approach
		if status.Succeeded > 0 {
			fmt.Printf("Job %s completed successfully (via status count)\n", jobName)
			return nil
		}

		if status.Failed > 0 {
			// Get more detailed error information
			cmd := exec.Command(kubectlPath,
				"--token="+token,
				"--server=https://cci."+appConfig.ProjectName+".myhuaweicloud.com",
				"--insecure-skip-tls-verify=true",
				"--v=0",
				"describe", "job/"+jobName,
				"--namespace="+appConfig.K8sNamespace,
			)
			out, _ := cmd.CombinedOutput()

			return fmt.Errorf("job %s failed (via status count)\nJob details:\n%s", jobName, string(out))
		}

		if status.Active == 0 && status.Succeeded == 0 && status.Failed == 0 {
			// Job might be pending or in an unknown state - but only report if condition check also failed
			if condErr != nil {
				fmt.Printf("Job %s is pending or in unknown state, continuing to wait...\n", jobName)
			} else {
				fmt.Printf("Job %s is running but not yet complete...\n", jobName)
			}
		} else {
			fmt.Printf("Job %s is still running (active: %d)...\n", jobName, status.Active)
		}

		// Wait before next poll
		time.Sleep(pollingInterval)
	}
}

func generateK8sJob(smnMessage sharedmodule.HostingDetail) ([]byte, error) {

	tmpl, err := template.ParseFS(kubernetesTemplates, "kubernetes-templates/hostingbuilder.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]string{
		"HostingBuilderImage": appConfig.HostingBuilderImage,
		"ImagePullPolicy":     appConfig.ImagePullPolicy,
		"JobName":             "hb-" + smnMessage.SubDomain,
		"Namespace":           appConfig.K8sNamespace,
		"LXD_HOST":            appConfig.LxdHost,
		"ANSIBLE_USER":        appConfig.AnsibleUser,
		"ANSIBLE_PASSWORD":    appConfig.AnsiblePassword,
		"SUBDOMAIN":           smnMessage.SubDomain,
		"EMAIL":               smnMessage.Email,
		"DB_ROOT_HOST":        appConfig.DbRootHost,
		"DB_ROOT_USER":        appConfig.DbRootUser,
		"DB_ROOT_PASSWORD":    appConfig.DbRootPassword,
		"WORDPRESS_THEME":     string(smnMessage.Theme),
		"ACCESS_KEY":          appConfig.AccessKey,
		"SECRET_KEY":          appConfig.SecretKey,
		"TOPIC_URN":           appConfig.TopicUrn,
	}); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	// Debug functionality
	if appConfig.PrintOutFile {

		debugFile, err := os.Create("./debug-telegram-job.yaml")

		if err != nil {
			// Don't fail the main operation if debug file creation fails
			fmt.Printf("Warning: failed to create debug file: %v\n", err)
		}

		debugFile.Write(buf.Bytes())
		fmt.Println("Debug file written to:", "./debug-telegram-job.yaml")
		defer debugFile.Close()
	}

	return buf.Bytes(), nil
}

type handler struct {
	K8sToken string
}

func (h *handler) SmnTrigger(payload []byte, ctx fgcontext.RuntimeContext) (interface{}, error) {
	var smnEvent smn.SMNTriggerEvent
	err := json.Unmarshal(payload, &smnEvent)
	if err != nil {
		slog.Error("unmarshal payload failed")
		return "invalid data", err
	}
	ctx.GetLogger().Logf("payload:%s", smnEvent.String())

	var c int = 1
	for _, record := range smnEvent.Record {
		var smnMessage sharedmodule.HostingDetail
		err = json.Unmarshal([]byte(record.Smn.Message), &smnMessage)
		if err != nil {
			slog.Error("unmarshal record failed")
			return "invalid data", err
		}

		yamlContent, err := generateK8sJob(smnMessage)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return nil, err
		}

		if err := applyK8s(h.K8sToken, yamlContent); err != nil {
			fmt.Printf("Error: %v\n", err)
			return nil, err
		}

		if err := waitK8sJobCompletion(h.K8sToken, "hb-"+smnMessage.SubDomain); err != nil {
			fmt.Printf("Error: %v\n", err)
			return nil, err
		}

		slog.Info(fmt.Sprintf("hosting builder job created #%d for %s", c, smnMessage.SubDomain))
		c++

		slog.Info("Continue send notification")

		// only process one record at a time
		if c == 1 {
			break
		}
	}

	slog.Info(fmt.Sprintf("Processed %d records", c-1))
	slog.Info("===========")

	return "ok", nil
}

func main() {

	loadConfig()

	token, err := getCCIToken()
	if err != nil {
		slog.Error("failed to get CCI token", slog.String("error", err.Error()))
		os.Exit(1)
	}

	h := &handler{
		K8sToken: token,
	}
	runtime.Register(h.SmnTrigger)
}
