package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	mathrand "math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"database/sql"

	"github.com/aws/aws-lambda-go/lambda"
	_ "github.com/go-sql-driver/mysql"
	"github.com/marv2097/siprocket"
)

//New Session ...
func New() (s *Session) {
	s = new(Session)
	s.conn = new(net.UDPConn)
	s.laddr = new(net.UDPAddr)
	s.raddr = new(net.UDPAddr)
	return
}

//Session ...
type Session struct {
	conn  *net.UDPConn
	laddr *net.UDPAddr
	raddr *net.UDPAddr
}

//SlackMessage ...
type SlackMessage struct {
	Text        string       `json:"text"`
	Username    string       `json:"username"`
	Channel     string       `json:"channel"`
	IconEmoji   string       `json:"icon_emoji"`
	Attachments []Attachment `json:"attachments"`
}

//Attachment ...
type Attachment struct {
	Text  string `json:"text"`
	Color string `json:"color"`
	Title string `json:"title"`
}

//ErrResp ...
type ErrResp struct {
	status bool   //true sucess , false error
	msg    string // error message
	code   int    // 0 sucess, 1 session error, 2 sip timeout
}

var ips = os.Getenv("SIP_DEST_HOST")  //sip servers
var port = os.Getenv("SIP_DEST_PORT") // 5060
var env = os.Getenv("environment")
var lh, lp string
var rip, rhost []string
var rpt int
var err error
var timeout chan bool
var ch chan []byte
var message string
var retries = 5
var dbURL = "dburl/opensips"

func main() {

	lambda.Start(Handler)

}

//Handler AWS Lambda entry point
func Handler() {

	if env == "Dev" {
		fmt.Println(ts() + "Dev mode: Testing Internet Access")
		if !testConn() {
			fmt.Println(ts() + "Dev mode: Failed test to Slack")
			os.Exit(1)
		}
	}

	rip = strings.Split(ips, ",")
	rhost = strings.Split(ips, ",")

	rip, rhost = getMediaServers(rip, rhost)

	for i, val := range rhost {
		rip[i] = resolveHost(val)
	}
	//fmt.Println(rip)
	if env == "Dev" {
		SendAlertToSlack(ts() + " Dev mode: Testing SlackSend Function")
	}

	rpt, _ = strconv.Atoi(port)
	if rpt == 0 {
		rpt = 5060
	}

	timeout = make(chan bool, 1)
	ch = make(chan []byte)

	for k := range rip {
		for i := 1; i <= retries; i++ {
			fmt.Println(ts()+"Message Attempt# ", i)
			resp := Send(rip[k], rhost[k])
			if !resp.status {
				if resp.code == 2 {
					if i == 5 {
						fmt.Println(i)
						SendAlertToSlack(env + ": ``` " + ts() + " SIP Request Timeout: " + rhost[k] + " (" + rip[k] + ")```")
					}
				}
			} else {
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
	}

}

//Send ...
func Send(ip string, host string) *ErrResp {
	s := New()
	s.raddr.IP = net.ParseIP(ip)
	s.raddr.Port = rpt
	s.laddr.IP = nil
	s.laddr.Port = 0

	s.conn, err = net.DialUDP("udp", s.laddr, s.raddr)
	if err != nil {
		fmt.Print(err)
		return parseResp(false, "Error in DialUDP", 1)

	}

	var data = "."
	_, err = s.conn.Write([]byte(data))
	if err != nil {
		fmt.Print(ts() + "Error initializing keepalive packet " + fmt.Sprint(err))
		return parseResp(false, "Error Initializing keeplive", 1)
	}

	lh, lp, err = net.SplitHostPort(s.conn.LocalAddr().String())
	if err != nil {
		fmt.Println(err)
	}

	m := make(map[string]string)
	m["{{.dst-ip}}"] = ip
	m["{{.dst-port}}"] = strconv.Itoa(rpt)
	m["{{.src-ip}}"] = lh
	m["{{.src-port}}"] = lp
	m["{{.callid}}"] = getRand(20)
	m["{{.cseq}}"] = getCseq(999)

	fmt.Printf(ts()+"Sending SIP Option to %s(%s)\n\n", host, s.raddr.IP)

	data = Option()
	for k, v := range m {
		data = strings.Replace(data, k, v, -1)
	}

	fmt.Print(data)

	_, err = s.conn.Write([]byte(data))
	if err != nil {
		fmt.Print(ts() + "Error writing UDP SIP Msg " + fmt.Sprint(err))
		return parseResp(false, "Error writing UDP SIP Msg", 1)
	}

	go Recv(s)

	go func() {
		time.Sleep(500 * time.Millisecond)
		timeout <- true
	}()

	select {
	case result := <-ch:
		// a read from ch has occurred
		ParseResult(result)
	case <-timeout:
		// the read from ch has timed out
		fmt.Println(ts() + env + "Debug channel timeout- SIP Request Timeout -" + host)
		return parseResp(false, "SIP Request Timeout", 2)
	}
	return parseResp(true, "Sucess", 0)
}

//Recv ...
func Recv(r *Session) {
	var buf [2000]byte

	n, _, err := r.conn.ReadFromUDP(buf[0:])
	if err != nil {
		fmt.Print(ts() + "Error on ReadFromUDP  " + fmt.Sprint(err))
	} else {
		fmt.Print(ts() + "ReadFromUDP: \n " + string(buf[0:n]))
		ch <- buf[0:n]
	}

}

//ParseResult ...
func ParseResult(result []byte) {
	sip := siprocket.Parse(result)
	fmt.Println("Response from:   " + string(sip.To.Host) + " " + string(sip.Req.Src))

	if string(sip.Req.StatusCode) != "200" {
		SendAlertToSlack(env + ": SIP Resp was not 200OK " + string(sip.Req.StatusCode) + " " + string(sip.To.Host))
	}

}

//SendAlertToSlack ...
func SendAlertToSlack(msg string) {

	client := &http.Client{}

	data := new(SlackMessage)
	data.Text = msg
	data.Channel = os.Getenv("SLACK_CHANNEL")
	data.IconEmoji = ":fire:"
	data.Username = os.Getenv("SLACK_USER")
	data.Attachments = []Attachment{}

	payload, err := json.Marshal(data)
	if err != nil {
		fmt.Print("SendAlertToSlack: " + fmt.Sprint(err))
	}

	webhookURL := "https://hooks.slack.com/services/" + os.Getenv("SLACK_WEBHOOK_TOKEN")

	req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(payload))
	if err != nil {
		fmt.Print("SendAlertToSlack: " + fmt.Sprint(err))
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Print("SendAlertToSlack: " + fmt.Sprint(err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Println(resp.StatusCode)
		fmt.Print("SendAlertToSlack: " + fmt.Sprint(err))
	}

}

//Option ...
func Option() string {

	return fmt.Sprint("OPTIONS sip:1234@{{.dst-ip}}:{{.dst-port}} SIP/2.0\r\n" +
		"Via: SIP/2.0/UDP {{.src-ip}}:{{.src-port}};branch=z9hG4bK29c68612;rport\r\n" +
		"From: \"cool-pinger\" <sip:coolpinger@{{.src-ip}}:{{.src-port}}>;tag=11122233\r\n" +
		"To: <sip:1234@{{.dst-ip}}:{{.dst-port}}>\r\n" +
		"Call-ID: {{.callid}}@{{.src-ip}}:{{.src-port}}\r\n" +
		"CSeq: {{.cseq}} OPTIONS\r\n" +
		"Contact: <sip:coolpinger@{{.src-ip}}:{{.src-port}}>\r\n" +
		"Content-Length: 0\r\n" +
		"Max-Forwards: 70\r\n" +
		"User-Agent: Cool Pinger\r\n" +
		"Accept: text/plain\r\n\r\n")
}

func resolveHost(host string) string {
	ips, err := net.LookupIP(host)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s : Could not get IPs: %v\n", env, err)
		os.Exit(1)
	}
	return fmt.Sprint(ips[0])
}

func getRand(n int) string {
	const alphanum = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	var bytes = make([]byte, n)
	rand.Read(bytes)
	for i, b := range bytes {
		bytes[i] = alphanum[b%byte(len(alphanum))]
	}
	return string(bytes)
}

func getCseq(n int) string {
	mathrand.Seed(time.Now().UnixNano())
	return strconv.Itoa(mathrand.Intn(n))
}

func parseResp(status bool, msg string, code int) *ErrResp {
	resp := new(ErrResp)
	resp.status = status
	resp.msg = msg
	resp.code = code
	return resp
}

func ts() string {
	return "- " + time.Now().UTC().String() + " -"
}

func getMediaServers(rip, rhost []string) ([]string, []string) {
	db, err := sql.Open("mysql", dbURL)
	if err != nil {
		fmt.Println(ts() + fmt.Sprint(err))
		return rip, rhost
	}
	defer db.Close()

	results, err := db.Query("select dst_uri from load_balancer")
	if err != nil {
		fmt.Println(ts() + fmt.Sprint(err))
		return rip, rhost
	}

	for results.Next() {
		var dstURI string

		err = results.Scan(&dstURI)
		if err != nil {
			fmt.Println(ts() + fmt.Sprint(err))
			return rip, rhost
		}

		dst := strings.Split(dstURI, ":")
		if strings.Contains(dstURI, "@") {
			dst = strings.Split(dstURI, "@")
		}

		rip = append(rip, dst[1])
		rhost = append(rhost, dst[1])

		fmt.Println(env + ts() + " getMediaServers: " + fmt.Sprint(dst[1]))
		fmt.Println(env + ts() + " getMediaServers: " + fmt.Sprint(rip))
	}
	return rip, rhost
}
