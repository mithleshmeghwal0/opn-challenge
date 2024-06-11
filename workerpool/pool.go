package workerpool

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"example.com/challenge/models"
	"github.com/omise/omise-go"
	"golang.org/x/time/rate"

	tomise "example.com/challenge/omisethrottled"
)

type WorkerPool struct {
	ctx         context.Context
	client      *tomise.Client
	jobs        chan *models.Record
	results     chan *models.Record
	retryJobs   chan *models.Record
	workerCount int
	ratelimit   *rate.Limiter
}

func NewWorkerPool(ctx context.Context, client *tomise.Client, recordsLen int, workerCount int, rateLimit *rate.Limiter) *WorkerPool {
	jobs := make(chan *models.Record, recordsLen)
	results := make(chan *models.Record, recordsLen)
	retryJobs := make(chan *models.Record, 2*recordsLen)

	pool := &WorkerPool{
		ctx:         ctx,
		client:      client,
		jobs:        jobs,
		results:     results,
		retryJobs:   retryJobs,
		workerCount: workerCount,
		ratelimit:   rateLimit,
	}

	go pool.requeueJobs()

	for i := 0; i < pool.workerCount; i++ {
		go pool.charge()
	}

	return pool
}

func (p *WorkerPool) ProcessRecords(records []*models.Record) {
	for idx, record := range records {
		record.Idx = idx
		p.jobs <- record
	}
}

func (p *WorkerPool) GetResults(recordsLen int) []*models.Record {
	var results []*models.Record
	for i := 0; i < recordsLen; i++ {
		results = append(results, <-p.results)
	}
	return results
}

func (p *WorkerPool) Close() {
	close(p.retryJobs)
	close(p.jobs)
	close(p.results)
}

func (p *WorkerPool) charge() {
	for record := range p.jobs {
		if p.client.IsThrottled() {
			p.retryJobs <- record
			continue
		}

		if err := p.ratelimit.Wait(p.ctx); err != nil {
			p.retryJobs <- record
			continue
		}

		// creating token
		token, err := p.client.CreateToken(record)
		if err != nil {
			fmt.Println(err)

			var e *omise.Error
			if errors.As(err, &e) && e.StatusCode == http.StatusTooManyRequests {
				p.client.Throttle()
				p.retryJobs <- record
				continue
			}

			record.Error = err
			p.results <- record
			continue
		}

		if err := p.ratelimit.Wait(p.ctx); err != nil {
			p.retryJobs <- record
			continue
		}

		// creating charge
		if err := p.client.CreateCharge(token, record); err != nil {
			fmt.Println(err)

			var e *omise.Error
			if errors.As(err, &e) && e.StatusCode == http.StatusTooManyRequests {
				p.client.Throttle()
				p.retryJobs <- record
				continue
			}

			record.Error = err
			p.results <- record
			continue
		}

		p.results <- record
	}
}

func (p *WorkerPool) requeueJobs() {
	for r := range p.retryJobs {
		p.jobs <- r
	}
}
