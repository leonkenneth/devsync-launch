package main

import (
	"github.com/gorilla/websocket"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"sync"
	"time"
)

func applyDiff(filename string, content string) {
	ioutil.WriteFile(path.Join("/app/", filename), []byte(content), 0644)
}

type FileChange struct {
	FileName string `json:"filename"`
	Content  string `json:"content"`
}

type Subscription struct {
	Action string `json:"action"`
	Token  string `json:"token"`
}

func token() string {
	return os.Getenv("DEVSYNC")
}

func main() {
	var wg sync.WaitGroup

	restart := make(chan bool)

	wg.Add(2)

	log.Println("Connecting to channel", token())
	go func() {
		runClient(token(), restart)
	}()

	// Run original program, store PID
	go func() {
		defer wg.Done()

		launchOriginal(restart)
	}()

	wg.Wait()
	time.Sleep(time.Second * 10)
}

func launchOriginal(restart chan bool) {
	launch(os.Args[1:], restart)
}

func launch(args []string, restart chan bool) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = os.Environ()

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.Start()

	go func() {
		r := <-restart

		if r {
			err := cmd.Process.Signal(os.Kill)
			if err != nil {
				log.Println(err)
			}

			launch(args, restart)
		}
	}()

	cmd.Wait()
}

func runClient(token string, restart chan bool) {
	url, _ := url.Parse("ws://mighty-stream-1810.herokuapp.com")
	conn, _ := net.Dial("tcp", "mighty-stream-1810.herokuapp.com:80")
	c, _, _ := websocket.NewClient(conn, url, http.Header{}, 1024, 1024)
	// TODO: Error handling

	c.WriteJSON(Subscription{
		Token:  token,
		Action: "subscribe",
	})

	for {
		fileChange := FileChange{
			FileName: "",
			Content:  "",
		}

		err := c.ReadJSON(&fileChange)

		if err != nil {
			log.Println(err)
		} else if fileChange.FileName == "." && fileChange.Content == "." {
			// Heartbeat
		} else {
			applyDiff(fileChange.FileName, fileChange.Content)
			restart <- true
		}
	}

}
