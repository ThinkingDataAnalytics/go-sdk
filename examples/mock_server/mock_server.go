package mock_server

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

func Start(action func()) {
	http.HandleFunc("/test", func(writer http.ResponseWriter, request *http.Request) {
		resMsg := fmt.Sprintf("-begin: %v\n", time.Now())

		action()

		resMsg += fmt.Sprintf("---end: %v\n", time.Now())

		fmt.Printf(resMsg)
		writer.Write([]byte(resMsg))
	})
	log.Println("listen server: http:/localhost:18081/test")
	err := http.ListenAndServe(":18081", nil)
	if err != nil {
		log.Println(err)
		return
	}
}
