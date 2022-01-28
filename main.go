package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-co-op/gocron"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

const (
	StatusOnline       = "online"
	StatusAway         = "away"
	StatusOffline      = "offline"
	StatusDoNotDisturb = "dnd"
)

type statusUpdate struct {
	Cron   string
	Status string
}

var (
	mattermostUrl         string
	mattermostAccessToken string
	statusUpdates         []statusUpdate
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

	err := viper.UnmarshalKey("status-updates", &statusUpdates)
	if err != nil {
		log.Fatalf("unable to read status updates: %s", err)
	}
	if len(statusUpdates) == 0 {
		log.Fatalf("empty status updates")
	}
}

///////////////////////////////////////////////////////////////////////////////

func main() {
	userInfo, err := userInfo()
	if err != nil {
		log.Fatalf("user info failed: %s", err)
	}

	scheduler := gocron.NewScheduler(time.UTC)
	for _, update := range statusUpdates {
		scheduleStatusUpdate(scheduler, userInfo.Id, update)
	}
	scheduler.StartBlocking()
}

func scheduleStatusUpdate(scheduler *gocron.Scheduler, userId string, update statusUpdate) {
	_, err := scheduler.CronWithSeconds(update.Cron).Do(func() {
		log.Infof("update status to %s", update.Status)
		err := updateStatus(userId, update.Status)
		if err != nil {
			log.Errorf("change status failed: %s, %s", update, err)
		}
	})
	if err != nil {
		log.Fatalf("unable to create cron job: %s, %s", update, err)
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
	response, err := mattermostApiRequest(http.MethodGet, "users/me", nil)
	if err != nil {
		return nil, err
	}

	var userResponseData userResponse
	err = json.Unmarshal(response, &userResponseData)
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

	_, err = mattermostApiRequest(http.MethodPut, "users/me/status", bytes.NewBuffer(updateRequestJson))

	return err
}

func mattermostApiRequest(method string, apiUrlPart string, body io.Reader) ([]byte, error) {
	apiUrl := fmt.Sprintf("%s/api/v4/%s", mattermostUrl, apiUrlPart)
	request, _ := http.NewRequest(method, apiUrl, body)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", mattermostAccessToken))

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("%s non-ok status code received: %s", apiUrlPart, response.Status))
	}

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return responseBody, nil
}
