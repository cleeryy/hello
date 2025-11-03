package monitor

import (
	"log"
	"sync"
	"time"

	"github.com/cleeryy/hello/internal/ping"
	"github.com/cleeryy/hello/internal/storage"
)

type Monitor struct {
	store  *storage.Storage
	ticker *time.Ticker
	stopCh chan bool
	wg     sync.WaitGroup
}

func New(store *storage.Storage, interval time.Duration) *Monitor {
	return &Monitor{
		store:  store,
		ticker: time.NewTicker(interval),
		stopCh: make(chan bool),
	}
}

func (m *Monitor) Start() {
	m.wg.Add(1)
	go m.run()
	log.Println("🔍 Device monitor started")
}

func (m *Monitor) Stop() {
	m.ticker.Stop()
	m.stopCh <- true
	m.wg.Wait()
	log.Println("🛑 Device monitor stopped")
}

func (m *Monitor) run() {
	defer m.wg.Done()

	for {
		select {
		case <-m.stopCh:
			return
		case <-m.ticker.C:
			m.checkDevices()
		}
	}
}

func (m *Monitor) checkDevices() {
	devices := m.store.GetAll()

	for _, device := range devices {
		if device.IP == "" || !device.PingEnabled {
			continue
		}

		isUp := ping.PingHost(device.IP, 2*time.Second)

		newStatus := "down"
		if isUp {
			newStatus = "up"
		}

		if device.Status != newStatus {
			device.Status = newStatus
			device.LastSeen = time.Now().Unix()

			if err := m.store.Update(device.ID, device); err != nil {
				log.Printf("❌ Failed to update device %s: %v", device.ID, err)
			} else {
				log.Printf("✅ Device %s status changed to %s", device.ID, newStatus)
			}
		}
	}
}

