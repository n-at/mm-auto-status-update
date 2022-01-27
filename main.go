package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"io"
	"io/ioutil"
	"net/http"
	"os"
)

const (
	StatusOnline       = "online"
	StatusAway         = "away"
	StatusOffline      = "offline"
	StatusDoNotDisturb = "dnd"
)

var (
	mattermostUrl         string
	mattermostAccessToken string
)

func init() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)

	viper.SetConfigName("application")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Unable to read config file: %s", err)
	}

	mattermostUrl = viper.GetString("mattermost-url")
	if mattermostUrl == "" {
		log.Fatalf("mattermost url not found")
	}
	mattermostAccessToken = viper.GetString("access-token")
	if mattermostAccessToken == "" {
		log.Fatalf("mattermost access token not found")
	}
}

///////////////////////////////////////////////////////////////////////////////

func main() {
	userInfo, err := userInfo()
	if err != nil {
		log.Fatalf("user info failed: %s", err)
	}

	err = updateStatus(userInfo.Id, StatusAway)
	if err != nil {
		log.Fatalf("change status failed: %s", err)
	}
}

///////////////////////////////////////////////////////////////////////////////

type updateUserStatusRequest struct {
	UserId     string `json:"user_id"`
	Status     string `json:"status"`
	DndEndTime int    `json:"dnd_end_time"`
}

type userResponse struct {
	Id       string `json:"id"`
	UserName string `json:"username"`
}

func userInfo() (*userResponse, error) {
	request, _ := newRequest(http.MethodGet, "users/me", nil)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	var userResponseData userResponse
	err = json.Unmarshal(body, &userResponseData)
	if err != nil {
		return nil, err
	}

	return &userResponseData, nil
}

func updateStatus(userId, newStatus string) error {
	updateRequest := updateUserStatusRequest{UserId: userId, Status: newStatus}
	updateRequestJson, err := json.Marshal(updateRequest)
	if err != nil {
		return err
	}

	request, _ := newRequest(http.MethodPut, "users/me/status", bytes.NewBuffer(updateRequestJson))
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return errors.New(fmt.Sprintf("non-ok status code received: %s", response.Status))
	}

	return nil
}

func newRequest(method string, apiUrlPart string, body io.Reader) (*http.Request, error) {
	apiUrl := fmt.Sprintf("%s/api/v4/%s", mattermostUrl, apiUrlPart)
	request, err := http.NewRequest(method, apiUrl, body)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", mattermostAccessToken))
	return request, err
}
