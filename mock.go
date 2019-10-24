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

func (configuration OperatorConfiguration) print(title string) {
	klog.Info(title)
	for key, val := range configuration {
		klog.Info(key, "\t", val)
	}
	fmt.Println()
}

// Create original operator configuration.
func createOriginalConfiguration() OperatorConfiguration {
	var cfg = NewOperatorConfiguration()
	jsonStr := `
{"no_op":"?",
 "foo":"FOO1",
 "bar":"BAR1",
 "watch":[]}`
	cfg.fromJSON([]byte(jsonStr))
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

	defer response.Body.Close()
	body, _ := ioutil.ReadAll(response.Body)
	klog.Info("************ BODY ***************")
	klog.Info(response.StatusCode)
	klog.Info(string(body))
	klog.Info("************ BODY ***************")

	var c2 = NewOperatorConfiguration()
	c2.fromJSON(body)
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
