package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

type OperatorConfiguration map[string]string

func NewOperatorConfiguration() OperatorConfiguration {
	return make(map[string]string)
}

var configuration = NewOperatorConfiguration()

func (configuration OperatorConfiguration) mergeWith(other OperatorConfiguration) {
	for key, value := range other {
		_, found := configuration[key]
		if found {
			configuration[key] = value
		}
	}
}

func init() {
	var c1 = NewOperatorConfiguration()
	c1["a"] = "A"
	c1["foo"] = "FOO"
	c1["bar"] = "BAR"

	var c2 = NewOperatorConfiguration()
	c2["b"] = "B"
	c2["foo"] = "FOO2"
	c2["bar"] = "BAR2"

	c1.mergeWith(c2)
	fmt.Println(c1)
}

func main() {
	log.Println("Starting the service")

	url := "http://localhost:8080/"

	request, err := http.NewRequest("GET", url, nil)

	if err != nil {
		log.Println("Error: " + err.Error())
	}

	response, err := http.DefaultClient.Do(request)

	if err != nil {
		log.Println("Error: " + err.Error())
	}

	defer response.Body.Close()
	body, _ := ioutil.ReadAll(response.Body)

	fmt.Println(string(body))

	log.Println("Stopping the service")
}
