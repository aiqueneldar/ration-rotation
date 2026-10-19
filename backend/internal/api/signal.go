package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	signalcli "github.com/taigrr/signalcli"
)

// SignalClient wraps signalcli for sending message notifications via signal-cli jsonRPC API
type SignalServer struct {
	Endpoint   string `json:"endpoint"`
	Client     *signalcli.Client
	Account    string `json:"account"`
	Linked     bool   `json:"linked,omitempty"`
	Name       string `json:"name,omitempty"`
	LinkStatus string `json:"link-status,omitempty"`
}

type StartLinkRes struct {
	JsonRCP string `json:"jsonrpc"`
	Result  struct {
		DeviceLinkUri string `json:"deviceLinkUri"`
	} `json:"result"`
	ID int `json:"id"`
}

type FinishLinkReq struct {
	JsonRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	ID      int    `json:"id"`
	Params  struct {
		DeviceLinkUri string `json:"deviceLinkUri"`
		DeviceName    string `json:"deviceName"`
	} `json:"params"`
}

type FinishLinkRes struct {
	JsonRPC string `json:"jsonrpc"`
	Result  struct {
		Number string `json:"number"`
		ACI    string `json:"aci"`
	} `json:"result"`
	ID int `json:"id"`
}

// SendSMS sends a text message to the specified recipient via Signal
func (sc *SignalServer) Send(recipients []string, message string) error {
	if !sc.Linked {
		return fmt.Errorf("Signal Account not yet linked")
	}
	ctx := context.Background()

	// Create send parameters using signalcli API
	params := signalcli.SendParams{}
	if len(recipients) == 1 {
		params.Recipient = recipients[0]
	} else if len(recipients) > 1 {
		params.Recipients = recipients
	}
	params.Message = message

	// Send the message via signal-cli library (handles HTTP internally)
	result, err := sc.Client.Send(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to send SMS to %s: %w", recipients, err)
	}

	log.Printf("✅ Sent to %s at timestamp: %d\n", recipients, result.Timestamp)
	return nil
}

func (srv *SignalServer) LinkingHandlerStart(c *gin.Context) {
	srv.LinkStatus = "Linking"
	var accData struct {
		Account string `json:"account"`
	}
	if err := c.ShouldBindJSON(&accData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Bad account info for linking", "err": err})
	}

	if srv.Client == nil && srv.Endpoint != "" {
		srv.Client = signalcli.NewClient(srv.Endpoint, accData.Account)
	} else if srv.Endpoint == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "No signal-cli endpoint specified", "err": ""})
	}

	rawLinkRes, err := srv.Client.Call(c, "startLink", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Couldn't start linking with Signal", "err": err})
		return
	}
	var startLinkRes StartLinkRes
	if err := json.Unmarshal(rawLinkRes, &startLinkRes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Couldn't unmarshal startLink response", "err": err})
	}

	timeout, cancle := context.WithTimeout(c, time.Second*30)
	go srv.LinkingWorker(timeout, cancle, &startLinkRes)

	c.JSON(http.StatusOK, &startLinkRes)
}

	func (srv *SignalServer) LinkingWorker(c context.Context, cancle context.CancelFunc, startLinkRes *StartLinkRes) {
	defer cancle() // Cancle if we finihs early

	finishLinkReq := FinishLinkReq{
		JsonRPC: "2.0",
		Method:  "finishLink",
		Params: struct {
			DeviceLinkUri string "json:\"deviceLinkUri\""
			DeviceName    string "json:\"deviceName\""
		}{
			DeviceLinkUri: startLinkRes.Result.DeviceLinkUri,
			DeviceName:    "ration-rotation",
		},
		ID: startLinkRes.ID + 1,
	}

	rawLinkRes, err := srv.Client.Call(c, finishLinkReq.Method, finishLinkReq.Params)
	if err != nil {
		srv.LinkStatus = "Error"
		return
	}
	var finishLink FinishLinkRes
	for try := 1; try <= 300; try++ {
		if err := json.Unmarshal(rawLinkRes, &finishLink); err != nil {
			srv.LinkStatus = fmt.Sprintf("Linking %d", try)
		}

		if finishLink.Result.ACI != "" || finishLink.Result.Number != "" {
			break
		}

		// Sleep a second and try again
		time.Sleep(time.Second)
	}

	// We have either timedout or got an appropriate answer
	if finishLink.Result.ACI != "" {
		srv.Account = finishLink.Result.ACI
		srv.Linked = true
		srv.LinkStatus = "Success"
	} else if finishLink.Result.Number != "" {
		srv.Account = finishLink.Result.Number
		srv.Linked = true
		srv.LinkStatus = "Success"
	} else {
		log.Println("Linking with Signal timed out. __Not Linked__")
		srv.Linked = false
		srv.LinkStatus = "Failure"
	}
}

func (srv *SignalServer) LinkingStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": srv.LinkStatus, "is-linked": srv.Linked})
}
