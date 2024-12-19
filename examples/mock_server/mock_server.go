package mock_server

import (
	"fmt"
	"github.com/ThinkingDataAnalytics/go-sdk/v2/src/thinkingdata"
	"log"
	"net/http"
	"time"
)

func Start(te *thinkingdata.TDAnalytics) {
	http.HandleFunc("/track", func(writer http.ResponseWriter, request *http.Request) {
		resMsg := fmt.Sprintf("-begin: %v\n", time.Now())

		asyncExample(te, "a", "d", nil)

		resMsg += fmt.Sprintf("---end: %v\n", time.Now())

		fmt.Printf(resMsg)
		_, err := writer.Write([]byte(resMsg))
		if err != nil {
			return
		}
	})
	http.HandleFunc("/close", func(writer http.ResponseWriter, request *http.Request) {
		err := te.Close()
		if err != nil {
			_, write := writer.Write([]byte(err.Error()))
			if write != nil {
				return
			}
		}
		_, write := writer.Write([]byte("ok"))
		if write != nil {
			return
		}
	})
	log.Println("listen server: http:/localhost:18081/track")
	err := http.ListenAndServe(":18081", nil)
	if err != nil {
		log.Println(err)
		return
	}
}

func asyncExample(te *thinkingdata.TDAnalytics, accountId, distinctId string, properties map[string]interface{}) {
	for i := 0; i < 200; i++ {
		go func(index int, distinctId string) {
			for j := 0; j < 100; j++ {
				err := te.Track(accountId, distinctId, fmt.Sprintf("ab___%v___%v", index, j), properties)
				if err != nil {
					fmt.Println(err)
				}
			}
			err := te.Flush()
			if err == nil {

			}
		}(i, distinctId)
	}
}
