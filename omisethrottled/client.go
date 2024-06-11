package omisethrottled

import (
	"errors"
	"sync"
	"time"

	"example.com/challenge/models"
	"github.com/omise/omise-go"
	"github.com/omise/omise-go/operations"
)

const (
	City         = "Bangkok"
	PostalCode   = "10320"
	SecurityCode = "123"

	ChargeCurrency = "THB"
)

type Client struct {
	sync.Mutex
	*omise.Client
	throttle         bool
	throttleDuration time.Duration
}

func NewClient(publicKey, secretKey string, throttleDuration time.Duration) (*Client, error) {
	client, err := omise.NewClient(publicKey, secretKey)
	if err != nil {
		return nil, err
	}

	return &Client{
		Client:           client,
		throttle:         false,
		throttleDuration: throttleDuration,
	}, nil
}

func (c *Client) getThrottle() bool {
	defer c.Unlock()
	c.Lock()
	return c.throttle
}
func (c *Client) setThrottle(throttle bool) {
	defer c.Unlock()
	c.Lock()
	c.throttle = throttle
}

func (c *Client) IsThrottled() bool {
	return c.getThrottle()
}

func (c *Client) Throttle() {
	c.setThrottle(true)
	go c.UnThrottle()
}

func (c *Client) UnThrottle() {
	time.Sleep(c.throttleDuration)
	c.setThrottle(false)
}

func (c *Client) CreateToken(record *models.Record) (string, error) {
	card := &omise.Card{}
	err := c.Do(card, &operations.CreateToken{
		Name:            record.Name,
		Number:          record.CCNumber,
		ExpirationMonth: time.Month(record.ExpMonth),
		ExpirationYear:  record.ExpYear,
		City:            City,
		PostalCode:      PostalCode,
		SecurityCode:    SecurityCode,
	})
	return card.ID, err
}

func (c *Client) CreateCharge(token string, record *models.Record) error {
	charge := &omise.Charge{}
	err := c.Do(charge, &operations.CreateCharge{
		Amount:   record.AmountSubunits,
		Currency: ChargeCurrency,
		Card:     token,
	})

	if err != nil {
		return err
	}

	if charge.Status != "successful" {
		return errors.New("charge failed")
	}

	return nil
}
