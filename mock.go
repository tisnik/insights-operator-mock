/*
Copyright © 2019 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package main

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/viper"
	"io"
	"io/ioutil"
	"k8s.io/klog"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"
)

// An unstructured operator configuration that can contain
// any data stored under (string) keys.
type OperatorConfiguration map[string]interface{}

// Constructor for the operator configuration.
func NewOperatorConfiguration() OperatorConfiguration {
	return make(map[string]interface{})
}

type Trigger struct {
	Id          int    `json:"id"`
	Type        string `json:"type"`
	Cluster     string `json:"cluster"`
	Reason      string `json:"reason"`
	Link        string `json:"link"`
	TriggeredAt string `json:"triggered_at"`
	TriggeredBy string `json:"triggered_by"`
	Parameters  string `json:"parameters"`
	Active      int    `json:"active"`
}

var configurationMutex sync.Mutex

var configuration = NewOperatorConfiguration()

func init() {
}

func (configuration OperatorConfiguration) fromJSON(payload []byte) error {
	return json.Unmarshal(payload, &configuration)
}

func (configuration OperatorConfiguration) addAll(other OperatorConfiguration) {
	for key, value := range other {
		configuration[key] = value
	}
}

func (configuration OperatorConfiguration) updateExisting(other OperatorConfiguration) {
	for key, value := range other {
		_, found := configuration[key]
		if found {
			configuration[key] = value
		}
	}
}

func (configuration OperatorConfiguration) mergeWith(other OperatorConfiguration) {
	if len(configuration) == 0 {
		configuration.addAll(other)
	} else {
		configuration.updateExisting(other)
	}
}

// Print the configuration. Items are sorted by its keys.
func (configuration OperatorConfiguration) print(title string) {
	klog.Info(title)
	if len(configuration) == 0 {
		klog.Info("\t* empty *")
		return
	}

	var keys []string
	for key, _ := range configuration {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		klog.Info("\t", key, "\t=> ", configuration[key])
	}
}

// Create original operator configuration.
func createOriginalConfiguration(filename string) OperatorConfiguration {
	var cfg = NewOperatorConfiguration()

	payload, err := ioutil.ReadFile(filename)
	if err != nil {
		klog.Error("Can not open configuration file: ", err)
		// ok for now, the configuration will be simply empty
		return cfg
	}

	err = cfg.fromJSON(payload)
	if err != nil {
		klog.Warning("Can not decode original configuration read from the file ", filename)
		// ok for now, the configuration will be simply empty
		return cfg
	}
	return cfg
}

func performReadRequest(url string) ([]byte, error) {
	response, err := http.Get(url)
	if err != nil {
		klog.Error("Communication error with the server", err)
		return nil, fmt.Errorf("Communication error with the server %v", err)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Expected HTTP status 200 OK, got %d", response.StatusCode)
	}
	body, readErr := ioutil.ReadAll(response.Body)
	defer response.Body.Close()

	if readErr != nil {
		return nil, fmt.Errorf("Unable to read response body")
	}

	return body, nil
}

func performWriteRequest(url string, method string, payload io.Reader) error {
	var client http.Client

	request, err := http.NewRequest(method, url, payload)
	if err != nil {
		return fmt.Errorf("Error creating request %v", err)
	}

	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("Communication error with the server %v", err)
	}
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusAccepted {
		return fmt.Errorf("Expected HTTP status 200 OK, 201 Created or 202 Accepted, got %d", response.StatusCode)
	}
	return nil
}

func retrieveConfigurationFrom(url string, cluster string) (OperatorConfiguration, error) {
	address := url + "/api/v1/operator/configuration/" + cluster

	body, err := performReadRequest(address)
	if err != nil {
		return nil, err
	}

	var c2 = NewOperatorConfiguration()
	err = c2.fromJSON(body)
	if err != nil {
		klog.Warning("Can not decode the configuration provided by the service")
		return nil, err
	}
	return c2, nil
}

func retrieveTriggersFrom(url string, cluster string) ([]Trigger, error) {
	address := url + "/api/v1/operator/triggers/" + cluster

	body, err := performReadRequest(address)
	if err != nil {
		return nil, err
	}

	var triggers []Trigger
	err = json.Unmarshal(body, &triggers)
	if err != nil {
		return nil, err
	}
	return triggers, nil
}

func ackTrigger(url string, cluster string, trigger int) error {
	address := url + "/api/v1/operator/trigger/" + cluster + "/ack/" + strconv.Itoa(trigger)

	err := performWriteRequest(address, "PUT", nil)
	if err != nil {
		klog.Error("unable to ack trigger")
	} else {
		klog.Info("trigger has been acked")
	}
	return err
}

func configurationGoroutine(serviceUrl string, configInterval int, clusterName string, configFile string) {
	klog.Info("Read original configuration")
	c1 := createOriginalConfiguration(configFile)
	c1.print("Original configuration")
	klog.Info("Gathering configuration each ", configInterval, " second(s)")
	for {
		klog.Info("Gathering info from service ", serviceUrl)
		c2, err := retrieveConfigurationFrom(serviceUrl, clusterName)
		if err != nil {
			klog.Error("unable to retrieve configuration from the service")
		} else if c2 != nil {
			c2.print("Retrieved configuration")
			configurationMutex.Lock()
			c1.mergeWith(c2)
			configurationMutex.Unlock()
			c1.print("Updated configuration")
		}
		time.Sleep(time.Duration(configInterval) * time.Second)
	}
}

func triggerGoroutine(serviceUrl string, triggerInterval int, clusterName string) {
	klog.Info("Gathering triggers each ", triggerInterval, " second(s)")
	for {
		klog.Info("Gathering triggers from service ", serviceUrl)
		triggers, err := retrieveTriggersFrom(serviceUrl, clusterName)
		if err != nil {
			klog.Error("unable to retrieve triggers from the service")
		} else {
			klog.Info("Triggers for this operator")
			if len(triggers) > 0 {
				for _, trigger := range triggers {
					klog.Info("\tId: ", trigger.Id)
					klog.Info("\tType: ", trigger.Type)
					klog.Info("\tReason: ", trigger.Reason)
					klog.Info("\tLink: ", trigger.Link)
					klog.Info("\tTriggered at: ", trigger.TriggeredAt)
					klog.Info("\tTriggered by: ", trigger.TriggeredBy)
					klog.Info("\tParameters: ", trigger.Parameters)
				}
				klog.Info("Performing triggers and acking them")
				for _, trigger := range triggers {
					klog.Info("Acking trigger: ", trigger.Id)
					ackTrigger(serviceUrl, clusterName, trigger.Id)
				}
			} else {
				klog.Info("\tNone")
			}
		}
		time.Sleep(time.Duration(triggerInterval) * time.Second)
	}
}

func StartInstrumentation(serviceUrl string, configInterval int, triggerInterval int, clusterName string, configFile string) {
	go configurationGoroutine(serviceUrl, configInterval, clusterName, configFile)
	go triggerGoroutine(serviceUrl, triggerInterval, clusterName)
}

func main() {
	viper.SetConfigName("config")
	viper.AddConfigPath(".")

	// accept configuration provided via environment variables as well
	viper.AutomaticEnv()
	viper.SetEnvPrefix("INSIGHTS_OPERATOR")

	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
	StartInstrumentation(viper.GetString("URL"), viper.GetInt("config_interval"), viper.GetInt("trigger_interval"),
		viper.GetString("cluster"), viper.GetString("configfile"))
	c := make(chan interface{})
	<-c
}
