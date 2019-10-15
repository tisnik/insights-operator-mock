package main

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/viper"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type OperatorConfiguration map[string]interface{}

func NewOperatorConfiguration() OperatorConfiguration {
	return make(map[string]interface{})
}

var configuration = NewOperatorConfiguration()

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
	fmt.Println(title)
	for key, val := range configuration {
		fmt.Println(key, val)
	}
	fmt.Println()
}

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
	log.Println("Retrieving configuration from the service")

	address := url + "/api/v1/operator/configuration/" + cluster

	request, err := http.NewRequest("GET", address, nil)
	if err != nil {
		log.Println("Error: " + err.Error())
		return nil, err
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		log.Println("Error: " + err.Error())
		return nil, err
	}

	defer response.Body.Close()
	body, _ := ioutil.ReadAll(response.Body)

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
			log.Println("unable to retrieve configuration from the service")
		} else {
			c2.print("Retrieved configuration")

			c1.mergeWith(c2)
			c1.print("Updated configuration")
		}

		time.Sleep(5 * time.Second)
	}
}
