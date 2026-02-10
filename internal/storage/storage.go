package storage

import (
	"os"
	"sort"

	"github.com/pelletier/go-toml/v2"
)

type QueueList struct {
	Queues []string `toml:"queues"`
}

const defaultFilename = "queues.toml"

func LoadQueues() ([]string, error) {
	data, err := os.ReadFile(defaultFilename)
	if os.IsNotExist(err) {
		list := QueueList{Queues: []string{}}
		data, _ := toml.Marshal(list)
		if err := os.WriteFile(defaultFilename, data, 0644); err != nil {
			return nil, err
		}
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}

	var list QueueList
	if err := toml.Unmarshal(data, &list); err != nil {
		emptyList := QueueList{Queues: []string{}}
		data, _ := toml.Marshal(emptyList)
		if err := os.WriteFile(defaultFilename, data, 0644); err != nil {
			return nil, err
		}
		return []string{}, nil
	}

	return list.Queues, nil
}

func SaveQueues(newQueues []string) error {
	existing, err := LoadQueues()
	if err != nil {
		existing = []string{}
	}

	queueMap := make(map[string]bool)
	for _, q := range existing {
		queueMap[q] = true
	}
	for _, q := range newQueues {
		queueMap[q] = true
	}

	var merged []string
	for q := range queueMap {
		if q != "" {
			merged = append(merged, q)
		}
	}
	sort.Strings(merged)

	list := QueueList{Queues: merged}
	data, err := toml.Marshal(list)
	if err != nil {
		return err
	}

	return os.WriteFile(defaultFilename, data, 0644)
}

func AddQueue(queue string) error {
	return SaveQueues([]string{queue})
}
