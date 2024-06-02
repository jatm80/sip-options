package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

func testConn() bool {
	resp, err := http.Get("http://myexternalip.com/raw")
	if err != nil {
		fmt.Print(err)
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		fmt.Println(string(bodyBytes))
		return true
	}
	return true
}
