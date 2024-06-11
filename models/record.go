package models

type Record struct {
	Idx            int    `csv:"-"`
	Name           string `csv:"Name"`
	AmountSubunits int64  `csv:"AmountSubunits"`
	CCNumber       string `csv:"CCNumber"`
	CVV            string `csv:"CVV"`
	ExpMonth       int    `csv:"ExpMonth"`
	ExpYear        int    `csv:"ExpYear"`
	Error          error  `csv:"-"`
}
