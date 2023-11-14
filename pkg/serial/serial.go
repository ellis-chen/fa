package serial

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
)

// PostJSON post json
func PostJSON(url string, m map[string]string) (string, error) {
	var reader io.Reader
	if m == nil {
		reader = nil
	} else {
		jsonValue, err := json.Marshal(m)
		if err != nil {
			return "", err
		}
		reader = bytes.NewBuffer(jsonValue)
	}

	response, err := http.Post(url, "application/json", reader)
	if err != nil {
		log.Fatalln(err.Error())
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(response.Body)

	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatal(err)
	}

	return string(responseData), nil
}
