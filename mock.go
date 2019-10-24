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
	"io/ioutil"
	"k8s.io/klog"
	"net/http"
	"sort"
	"time"
)

// An unstructured operator configuration that can contain
// any data stored under (string) keys.
type OperatorConfiguration map[string]interface{}

// Constructor for the operator configuration.
func NewOperatorConfiguration() OperatorConfiguration {
	return make(map[string]interface{})
}

var configuration = NewOperatorConfiguration()

func init() {
}

func (configuration OperatorConfiguration) fromJSON(payload []byte) error {
	return json.Unmarshal(payload, &configuration)
}

func (configuration OperatorConfiguration) mergeWith(other OperatorConfiguration) {
	for key, value := range other {
		_, found := configuration[key]
		if found {
			configuration[key] = value
		}
	}
}

// Print the configuration. Items are sorted by its keys.
func (configuration OperatorConfiguration) print(title string) {
	klog.Info(title)
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
	payload, err := ioutil.ReadFile(filename)
	if err != nil {
		klog.Fatal(err)
	}

	var cfg = NewOperatorConfiguration()
	err = cfg.fromJSON(payload)
	if err != nil {
		klog.Warning("Can not decode original configuration read from the file ", filename)
		// ok for now, the configuration will be simply empty
	}
	return cfg
}

func retrieveConfigurationFrom(url string, cluster string) (OperatorConfiguration, error) {
	address := url + "/api/v1/operator/configuration/" + cluster

	request, err := http.NewRequest("GET", address, nil)
	if err != nil {
		klog.Error("Error: " + err.Error())
		return nil, err
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		klog.Error("Error: " + err.Error())
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		klog.Info("No configuration has been provided by the service")
		return nil, nil
	}

	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
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

func main() {
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}

	c1 := createOriginalConfiguration()
	c1.print("Original configuration")

	for {
		c2, err := retrieveConfigurationFrom(viper.GetString("URL"), viper.GetString("cluster"))
		if err != nil {
			klog.Error("unable to retrieve configuration from the service")
		} else {
			c2.print("Retrieved configuration")

			c1.mergeWith(c2)
			c1.print("Updated configuration")
		}

		time.Sleep(5 * time.Second)
	}
}
