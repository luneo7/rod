// Package cdp for application layer communication with browser.
package cdp

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/go-rod/rod/lib/defaults"
	"github.com/go-rod/rod/lib/utils"
)

// Request to send to browser
type Request struct {
	ID        int         `json:"id"`
	SessionID string      `json:"sessionId,omitempty"`
	Method    string      `json:"method"`
	Params    interface{} `json:"params,omitempty"`
}

// Response from browser
type Response struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *Error          `json:"error,omitempty"`
}

// Event from browser
type Event struct {
	SessionID string          `json:"sessionId,omitempty"`
	Method    string          `json:"method"`
	Params    json.RawMessage `json:"params,omitempty"`
}

// WebSocketable enables you to choose the websocket lib you want to use.
// Such as you can easily wrap gorilla/websocket and use it as the transport layer.
type WebSocketable interface {
	// Connect to server
	Connect(ctx context.Context, url string, header http.Header) error
	// Send text message only
	Send([]byte) error
	// Read returns text message only
	Read() ([]byte, error)
	// Close the connection
	Close() error
}

// Client is a devtools protocol connection instance.
type Client struct {
	wsURL  string
	header http.Header
	ws     WebSocketable

	pending sync.Map    // pending requests
	event   chan *Event // events from browser

	count uint64

	logger utils.Logger
}

// New creates a cdp connection, all messages from Client.Event must be received or they will block the client.
func New(websocketURL string) *Client {
	return &Client{
		event:  make(chan *Event),
		wsURL:  websocketURL,
		logger: defaults.CDP,
	}
}

// Header set the header of the remote control websocket request
func (cdp *Client) Header(header http.Header) *Client {
	cdp.header = header
	return cdp
}

// Websocket set the websocket lib to use
func (cdp *Client) Websocket(ws WebSocketable) *Client {
	cdp.ws = ws
	return cdp
}

// Logger sets the logger to log all the requests, responses, and events transferred between Rod and the browser.
// The default format for each type is in file format.go
func (cdp *Client) Logger(l utils.Logger) *Client {
	cdp.logger = l
	return cdp
}

// Connect to browser
func (cdp *Client) Connect(ctx context.Context) error {
	if cdp.ws == nil {
		cdp.ws = &WebSocket{}
	}

	err := cdp.ws.Connect(ctx, cdp.wsURL, cdp.header)
	if err != nil {
		return err
	}

	go cdp.consumeMessages()

	return nil
}

// Close the underlying websocket connection and make all the pending Client.Call return the reason immediately.
func (cdp *Client) Close(reason error) error {
	cdp.pending.Range(func(_, val interface{}) bool {
		val.(*utils.Promise).Done(nil, &errConnClosed{details: reason})
		return true
	})

	return cdp.ws.Close()
}

// MustConnect is similar to Connect
func (cdp *Client) MustConnect(ctx context.Context) *Client {
	utils.E(cdp.Connect(ctx))
	return cdp
}

// Call a method and wait for its response
func (cdp *Client) Call(ctx context.Context, sessionID, method string, params interface{}) ([]byte, error) {
	req := &Request{
		ID:        int(atomic.AddUint64(&cdp.count, 1)),
		SessionID: sessionID,
		Method:    method,
		Params:    params,
	}

	cdp.logger.Println(req)

	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	err = cdp.ws.Send(data)
	if err != nil {
		_ = cdp.Close(err)
		return nil, &errConnClosed{details: err}
	}

	p := utils.NewPromise()
	cdp.pending.Store(req.ID, p)
	defer cdp.pending.Delete(req.ID)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.Wait():
		val, err := p.Result()
		if err != nil {
			return nil, err
		}
		return val.(json.RawMessage), nil
	}
}

// Event returns a channel that will emit browser devtools protocol events. Must be consumed or will block producer.
func (cdp *Client) Event() <-chan *Event {
	return cdp.event
}

// Consume messages coming from the browser via the websocket.
func (cdp *Client) consumeMessages() {
	defer close(cdp.event)

	for {
		data, err := cdp.ws.Read()
		if err != nil {
			_ = cdp.Close(err)
			return
		}

		var id struct {
			ID int `json:"id"`
		}
		err = json.Unmarshal(data, &id)
		utils.E(err)

		if id.ID != 0 {
			var res Response
			err := json.Unmarshal(data, &res)
			utils.E(err)

			cdp.logger.Println(&res)

			if val, ok := cdp.pending.Load(id.ID); ok {
				if res.Error != nil {
					val.(*utils.Promise).Done(nil, res.Error)
				} else {
					val.(*utils.Promise).Done(res.Result, nil)
				}
			}
		} else {
			var evt Event
			err := json.Unmarshal(data, &evt)
			utils.E(err)
			cdp.logger.Println(&evt)
			cdp.event <- &evt
		}
	}
}
