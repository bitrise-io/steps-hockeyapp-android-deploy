package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

// -----------------------
// --- Constants
// -----------------------

const (
	hockeyAppDeployStatusKey     = "HOCKEYAPP_DEPLOY_STATUS"
	hockeyAppDeployStatusSuccess = "success"
	hockeyAppDeployStatusFailed  = "failed"
	hockeyAppDeployPublicURLKey  = "HOCKEYAPP_DEPLOY_PUBLIC_URL"
	hockeyAppDeployBuildURLKey   = "HOCKEYAPP_DEPLOY_BUILD_URL"
	hockeyAppDeployConfigURLKey  = "HOCKEYAPP_DEPLOY_CONFIG_URL"
)

// -----------------------
// --- Models
// -----------------------

// ResponseModel ...
type ResponseModel struct {
	ConfigURL string `json:"config_url"`
	PublicURL string `json:"public_url"`
	BuildURL  string `json:"build_url"`
}

// -----------------------
// --- Functions
// -----------------------

func logFail(format string, v ...interface{}) {
	if err := exportEnvironmentWithEnvman(hockeyAppDeployStatusKey, hockeyAppDeployStatusFailed); err != nil {
		logWarn("Failed to export %s, error: %s", hockeyAppDeployStatusKey, err)
	}

	errorMsg := fmt.Sprintf(format, v...)
	fmt.Printf("\x1b[31;1m%s\x1b[0m\n", errorMsg)
	os.Exit(1)
}

func logWarn(format string, v ...interface{}) {
	errorMsg := fmt.Sprintf(format, v...)
	fmt.Printf("\x1b[33;1m%s\x1b[0m\n", errorMsg)
}

func logInfo(format string, v ...interface{}) {
	fmt.Println()
	errorMsg := fmt.Sprintf(format, v...)
	fmt.Printf("\x1b[34;1m%s\x1b[0m\n", errorMsg)
}

func logDetails(format string, v ...interface{}) {
	errorMsg := fmt.Sprintf(format, v...)
	fmt.Printf("  %s\n", errorMsg)
}

func logDone(format string, v ...interface{}) {
	errorMsg := fmt.Sprintf(format, v...)
	fmt.Printf("  \x1b[32;1m%s\x1b[0m\n", errorMsg)
}

func genericIsPathExists(pth string) (os.FileInfo, bool, error) {
	if pth == "" {
		return nil, false, errors.New("No path provided")
	}
	fileInf, err := os.Stat(pth)
	if err == nil {
		return fileInf, true, nil
	}
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	return fileInf, false, err
}

// IsPathExists ...
func IsPathExists(pth string) (bool, error) {
	_, isExists, err := genericIsPathExists(pth)
	return isExists, err
}

func exportEnvironmentWithEnvman(keyStr, valueStr string) error {
	envman := exec.Command("envman", "add", "--key", keyStr)
	envman.Stdin = strings.NewReader(valueStr)
	envman.Stdout = os.Stdout
	envman.Stderr = os.Stderr
	return envman.Run()
}

func createRequest(url string, fields, files map[string]string) (*http.Request, error) {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	// Add fields
	for key, value := range fields {
		if err := w.WriteField(key, value); err != nil {
			return nil, err
		}
	}

	// Add files
	for key, file := range files {
		f, err := os.Open(file)
		if err != nil {
			return nil, err
		}
		fw, err := w.CreateFormFile(key, file)
		if err != nil {
			return nil, err
		}
		if _, err = io.Copy(fw, f); err != nil {
			return nil, err
		}
	}

	w.Close()

	req, err := http.NewRequest("POST", url, &b)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", w.FormDataContentType())

	return req, nil
}

// -----------------------
// --- Main
// -----------------------

func main() {
	//
	// Validate options
	apkPath := os.Getenv("apk_path")
	mappingPath := os.Getenv("mapping_path")
	apiToken := os.Getenv("api_token")
	appID := os.Getenv("app_id")
	notes := os.Getenv("notes")
	notesType := os.Getenv("notes_type")
	notify := os.Getenv("notify")
	status := os.Getenv("status")
	tags := os.Getenv("tags")
	commitSHA := os.Getenv("commit_sha")
	buildServerURL := os.Getenv("build_server_url")
	repositoryURL := os.Getenv("repository_url")

	// mandatory handling, with backward compatibility
	//  0 - not mandatory (default)
	//  1 - mandatory
	mandatory := "0"
	if os.Getenv("mandatory") == "1" || os.Getenv("mandatory") == "true" {
		mandatory = "1"
	}

	logInfo("Configs:")
	logDetails("apk_path: %s", apkPath)
	logDetails("mapping_path: %s", mappingPath)
	logDetails("api_token: ***")
	logDetails("app_id: %s", appID)
	logDetails("notes: %s", notes)
	logDetails("notes_type: %s", notesType)
	logDetails("notify: %s", notify)
	logDetails("status: %s", status)
	logDetails("mandatory: %s", mandatory)
	logDetails("tags: %s", tags)
	logDetails("commit_sha: %s", commitSHA)
	logDetails("build_server_url: %s", buildServerURL)
	logDetails("repository_url: %s", repositoryURL)

	if apkPath == "" {
		logFail("Missing required input: apk_path")
	}
	if exist, err := IsPathExists(apkPath); err != nil {
		logFail("Failed to check if apk (%s) exist, error: %#v", apkPath, err)
	} else if !exist {
		logFail("No apk found to deploy. Specified path was: %s", apkPath)
	}

	if mappingPath != "" {
		if exist, err := IsPathExists(mappingPath); err != nil {
			logFail("Failed to check if mapping (%s) exist, error: %#v", mappingPath, err)
		} else if !exist {
			logFail("No mapping found to deploy. Specified path was: %s", mappingPath)
		}
	}

	if apiToken == "" {
		logFail("No App api_token provided as environment variable. Terminating...")
	}

	//
	// Create request
	logInfo("Performing request")

	requestURL := "https://rink.hockeyapp.net/api/2/apps/upload"
	if appID != "" {
		requestURL = fmt.Sprintf("https://rink.hockeyapp.net/api/2/apps/%s/app_versions/upload", appID)
	}

	fields := map[string]string{
		"notes":            notes,
		"notes_type":       notesType,
		"notify":           notify,
		"status":           status,
		"mandatory":        mandatory,
		"tags":             tags,
		"commit_sha":       commitSHA,
		"build_server_url": buildServerURL,
		"repository_url":   repositoryURL,
	}

	files := map[string]string{
		"ipa": apkPath,
	}
	if mappingPath != "" {
		files["dsym"] = mappingPath
	}

	request, err := createRequest(requestURL, fields, files)
	if err != nil {
		logFail("Failed to create request, error: %#v", err)
	}
	request.Header.Add("X-HockeyAppToken", apiToken)

	client := http.Client{}
	response, requestErr := client.Do(request)

	defer response.Body.Close()
	contents, readErr := ioutil.ReadAll(response.Body)

	//
	// Process response

	// Error
	if requestErr != nil {
		if readErr != nil {
			logWarn("Failed to read response body, error: %#v", readErr)
		} else {
			logInfo("Response:")
			logDetails("status code: %d", response.StatusCode)
			logDetails("body: %s", string(contents))
		}
		logFail("Performing request failed, error: %#v", requestErr)
	}

	if response.StatusCode < 200 || response.StatusCode > 300 {
		if readErr != nil {
			logWarn("Failed to read response body, error: %#v", readErr)
		} else {
			logInfo("Response:")
			logDetails("status code: %d", response.StatusCode)
			logDetails("body: %s", string(contents))
		}
		logFail("Performing request failed, status code: %d", response.StatusCode)
	}

	// Success
	logDone("Request succed")

	logInfo("Response:")
	logDetails("status code: %d", response.StatusCode)
	logDetails("body: %s", contents)

	if readErr != nil {
		logFail("Failed to read response body, error: %#v", readErr)
	}

	var responseModel ResponseModel
	if err := json.Unmarshal([]byte(contents), &responseModel); err != nil {
		logFail("Failed to parse response body, error: %#v", err)
	}

	fmt.Println()
	if responseModel.PublicURL != "" {
		logDone("Public URL: %s", responseModel.PublicURL)
	}
	if responseModel.BuildURL != "" {
		logDone("Build (direct download) URL: %s", responseModel.BuildURL)
	}
	if responseModel.ConfigURL != "" {
		logDone("Config URL: %s", responseModel.ConfigURL)
	}

	if err := exportEnvironmentWithEnvman(hockeyAppDeployStatusKey, hockeyAppDeployStatusSuccess); err != nil {
		logFail("Failed to export %s, error: %#v", hockeyAppDeployStatusKey, err)
	}

	if err := exportEnvironmentWithEnvman(hockeyAppDeployPublicURLKey, responseModel.PublicURL); err != nil {
		logFail("Failed to export %s, error: %#v", hockeyAppDeployPublicURLKey, err)
	}

	if err := exportEnvironmentWithEnvman(hockeyAppDeployBuildURLKey, responseModel.BuildURL); err != nil {
		logFail("Failed to export %s, error: %#v", hockeyAppDeployBuildURLKey, err)
	}

	if err := exportEnvironmentWithEnvman(hockeyAppDeployConfigURLKey, responseModel.ConfigURL); err != nil {
		logFail("Failed to export %s, error: %#v", hockeyAppDeployConfigURLKey, err)
	}
}
