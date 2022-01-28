package controllers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	cachev1 "github.com/NetApp/astraagent-operator/api/v1"
)

func (r *AstraAgentReconciler) RegisterClient(m *cachev1.AstraAgent) (string, error) {
	natsSyncClientURL := fmt.Sprintf("http://%s.%s:%d/bridge-client/1", m.Spec.NatssyncClient.Name, m.Spec.Namespace, m.Spec.NatssyncClient.Port)
	natsSyncClientRegisterURL := fmt.Sprintf("%s/register", natsSyncClientURL)
	reqBodyBytes, err := json.Marshal(map[string]string{"authToken": m.Spec.Astra.Token})
	response, err := http.Post(natsSyncClientRegisterURL, "application/json", bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return "", err
	}

	if response.StatusCode != 201 {
		bodyBytes, err := io.ReadAll(response.Body)
		if err != nil {
			return "", err
		}
		errMsg := fmt.Sprintf("Unexpected registration status code: %d; %s", response.StatusCode, string(bodyBytes))
		return "", errors.New(errMsg)
	}
	type LocationId struct {
		LocationId string `json:"locationID"`
	}
	locationId := &LocationId{}
	err = json.NewDecoder(response.Body).Decode(locationId)
	if err != nil {
		return "", err
	}
	return locationId.LocationId, nil
}

