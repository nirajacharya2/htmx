package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"golang.org/x/net/websocket"
)

func getRoot(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./static/index.html")
}

func hi(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "<div>hi</div>")
}

func gets(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	storeTODO(r.FormValue("task"))
	fmt.Println(r.FormValue("task"))

	tastList := `
				<div hx-boost="true" class="new-element"> 
					<a href="/task/` + r.FormValue("text") + `">` + r.FormValue("task") + `</a>
					<form hx-trigger="click" hx-delete="/delete" hx-target="closest .new-element" hx-swap="outerHTML swap:1s">
						<input type="hidden" name="task" value="` + r.FormValue("task") + `"/>
						delete
					</form>
				</div>`
	io.WriteString(w, tastList)
}

func storeTODO(task string) {
	readFile, err := os.OpenFile("database.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, os.ModePerm)
	if err != nil {
		fmt.Println(err)
	}
	_, err = fmt.Fprintln(readFile, task)
	if err != nil {
		fmt.Println(err)
	}
	readFile.Close()
}

func listTask(w http.ResponseWriter, r *http.Request) {
	tastList := getTODO()
	io.WriteString(w, tastList)
}

func getTODO() string {
	readFile, err := os.Open("database.txt")
	task := ""
	if err != nil {
		fmt.Println(err)
	}
	fileScanner := bufio.NewScanner(readFile)

	fileScanner.Split(bufio.ScanLines)
	lineNum1 := 0
	for fileScanner.Scan() {
		lineNum1++
		task += `
		<div hx-boost="true" class="new-element"> 
			<a href="/task/` + fileScanner.Text() + `">` + fileScanner.Text() +`</a>
			<form hx-trigger="click" hx-delete="/delete" hx-target="closest .new-element" hx-swap="outerHTML swap:1s">
				<input  type="hidden" name="task" value="` + fileScanner.Text() + `"/>
				delete
			</form>
		</div>`
	}
	readFile.Close()
	return task
}

func getTaskPage(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	fmt.Println(path)
	segments := strings.Split(path, "/")

	var result []string
	for _, s := range segments {
		if s != "" {
			result = append(result, s)
		}
	}
	if len(result) > 0 {
		fmt.Print(result[1])
		taskName := result[1]

		fmt.Println(taskName)
		io.WriteString(w, `
							<!DOCTYPE html>
								<html lang="en">
									<head>
										<meta charset="UTF-8" />
										<meta http-equiv="X-UA-Compatible" content="IE=edge" />
										<script src="https://unpkg.com/htmx.org@1.9.5"></script>
										<meta name="viewport" content="width=device-width, initial-scale=1.0" />
										<link
										href="http://localhost:3333/style.css"
										rel="stylesheet"
										type="text/css"
										/>
										<title>Document</title>
									</head>
									<body>
										<div id="app" class="red">
										<div>`+taskName+`</div>
										<form
											id="myForm2"
											hx-put="/edit"
											hx-target="#app">
												<input type="text" name="task" required />
												<input type="hidden" name="old" value="`+result[1]+`"/>
												<input type="submit" value="edit" />
										</form>
										</div>
									</body>
								</html>
							`)
	}
}

func removeTask(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}
	asciiString := string(body)
	decodedValue, err := url.QueryUnescape(asciiString)
	t := strings.Replace(decodedValue, "task=", "", -1)
	deleteLineFromFile("database.txt", t)

	io.WriteString(w, "")
}

func deleteLineFromFile(filePath string, taskName1 string) error {
	file, err := os.OpenFile(filePath, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	tempFilePath := filePath + ".temp"
	tempFile, err := os.Create(tempFilePath)
	if err != nil {
		return err
	}
	defer tempFile.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == taskName1 {
			continue
		}
		fmt.Fprintln(tempFile, line)
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	file.Close()
	if err := os.Remove(filePath); err != nil {
		return err
	}
	if err := os.Rename(tempFilePath, filePath); err != nil {
		return err
	}

	return nil
}

type Headers struct {
	HXRequest     string `json:"HX-Request"`
	HXTrigger     string `json:"HX-Trigger"`
	HXTriggerName string `json:"HX-Trigger-Name"`
	HXTarget      string `json:"HX-Target"`
	HXCurrentURL  string `json:"HX-Current-URL"`
}

type Data struct {
	Task    string  `json:"task"`
	Headers Headers `json:"HEADERS"`
}

type Server struct {
	conns map[*websocket.Conn]bool
	mutex sync.Mutex
}

func NewServer() *Server {
	return &Server{
		conns: make(map[*websocket.Conn]bool),
		mutex: sync.Mutex{},
	}
}

func (s *Server) websocketHandler(ws *websocket.Conn) {
	defer ws.Close()
	fmt.Println("WebSocket connection established.")
	s.conns[ws] = true
	s.readLoop(ws)
}

func (s *Server) readLoop(ws *websocket.Conn) {
	defer ws.Close()
	fmt.Println("WebSocket connection established.")
	s.conns[ws] = true

	for {
		var message string
		err := websocket.Message.Receive(ws, &message)
		if err != nil {
			fmt.Println("Error receiving message:", err)
			delete(s.conns, ws)
			break
		}
		jsonBytes := []byte(message)
		var data Data
		if err := json.Unmarshal(jsonBytes, &data); err != nil {
			fmt.Println("Error unmarshaling JSON:", err)
			return
		}

		fmt.Println("Received message:", data.Task)
		storeTODO(data.Task)
		tastList := `
		<div hx-swap-oob="beforeend:#todo-list">
			<div hx-boost="true" class="new-element">
				<a href="/task/` + data.Task + `">` + data.Task + `</a>
				<form hx-trigger="click" hx-delete="/delete" hx-target="closest .new-element" hx-swap="outerHTML swap:1s>
					<input type="hidden" name="task" value="` + data.Task + `"/>
					delete
				</form>
			</div>
		</div>`
		s.broadcast(tastList)
	}
}

func (s *Server) broadcast(text string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for ws := range s.conns {
		go func(ws *websocket.Conn) {
			if _, err := ws.Write([]byte(text)); err != nil {
				fmt.Println("Error sending message:", err)
				ws.Close()
				delete(s.conns, ws)
			}
		}(ws)
	}
}

func main() {
	server := NewServer()

	http.HandleFunc("/", getRoot)
	http.HandleFunc("/save", gets)
	http.HandleFunc("/taskList", listTask)
	http.HandleFunc("/task/", getTaskPage)
	http.HandleFunc("/delete", removeTask)

	http.Handle("/ws", websocket.Handler(server.websocketHandler))

	http.ListenAndServe(":3333", nil)
}
