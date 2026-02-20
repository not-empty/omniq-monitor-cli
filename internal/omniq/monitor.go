package omniq

import (
	"context"
	"log"
	"sort"
	"strings"

	"github.com/redis/go-redis/v9"
)

type Counts struct {
	Paused    bool
	Waiting   int64
	Active    int64
	Delayed   int64
	Completed int64
	Failed    int64
}

type GroupStatus struct {
	GID      string
	Inflight int64
	Limit    int64
	Waiting  int64
}

type Monitor struct {
	client *Client
}

func NewMonitor(client *Client) *Monitor {
	return &Monitor{client: client}
}

func (m *Monitor) GetCounts(ctx context.Context, queue string) (Counts, error) {
	base := formatQueueName(queue)
	pipe := m.client.rdb.Pipeline()

	kPaused := base + ":paused"
	kWait := base + "*:wait"
	kDelayed := base + ":delayed"
	kActive := base + ":active"
	kCompleted := base + ":completed"
	kFailed := base + ":failed"

	cmdPaused := pipe.Exists(ctx, kPaused)
	var wait int64
	cursor := uint64(0)
	for {
		keys, cursor, err := m.client.rdb.Scan(ctx, cursor, kWait, 1000).Result()
		if err != nil {
			return Counts{}, err
		}
		for _, key := range keys {
			wait += m.client.rdb.LLen(ctx, key).Val()
		}
		if cursor == 0 {
			break
		}
	}
	cmdDelayed := pipe.ZCard(ctx, kDelayed)
	cmdActive := pipe.ZCard(ctx, kActive)
	cmdCompleted := pipe.LLen(ctx, kCompleted)
	cmdFailed := pipe.LLen(ctx, kFailed)

	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return Counts{}, err
	}

	return Counts{
		Paused:    cmdPaused.Val() > 0,
		Waiting:   wait,
		Active:    cmdActive.Val(),
		Delayed:   cmdDelayed.Val(),
		Completed: cmdCompleted.Val(),
		Failed:    cmdFailed.Val(),
	}, nil
}

func (m *Monitor) GetGroups(ctx context.Context, queue string, limit int, extraGIDs []string) ([]GroupStatus, error) {
	base := formatQueueName(queue)

	kGReady := base + ":groups:ready"

	groupsSlice, err := m.client.rdb.ZRange(ctx, kGReady, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}

	groupsMap := make(map[string]bool)
	for _, g := range groupsSlice {
		groupsMap[g] = true
	}
	for _, g := range extraGIDs {
		groupsMap[g] = true
	}

	var groups []string
	for g := range groupsMap {
		groups = append(groups, g)
	}
	sort.Strings(groups)

	if len(groups) == 0 {
		return []GroupStatus{}, nil
	}

	pipe := m.client.rdb.Pipeline()
	cmds := make(map[string]struct {
		Inflight *redis.StringCmd
		Limit    *redis.StringCmd
		Wait     *redis.IntCmd
	})

	for _, g := range groups {
		kInflight := base + ":g:" + g + ":inflight"
		kLimit := base + ":g:" + g + ":limit"
		kWait := base + ":g:" + g + ":wait"

		cmds[g] = struct {
			Inflight *redis.StringCmd
			Limit    *redis.StringCmd
			Wait     *redis.IntCmd
		}{
			Inflight: pipe.Get(ctx, kInflight),
			Limit:    pipe.Get(ctx, kLimit),
			Wait:     pipe.LLen(ctx, kWait),
		}
	}

	_, err = pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		log.Println("Failed to get group status:", err)
	}

	var statuses []GroupStatus
	for _, g := range groups {
		c := cmds[g]

		inflight, _ := c.Inflight.Int64()
		limitVal, _ := c.Limit.Int64()
		if limitVal == 0 {
			limitVal = 1
		}
		wait := c.Wait.Val()

		statuses = append(statuses, GroupStatus{
			GID:      g,
			Inflight: inflight,
			Limit:    limitVal,
			Waiting:  wait,
		})
	}

	return statuses, nil
}

func (c *Monitor) ScanQueues(ctx context.Context, matchPattern string) ([]string, error) {
	var queues []string
	scanMatch := matchPattern
	if scanMatch == "" {
		scanMatch = "*:wait"
	}

	iter := c.client.rdb.Scan(ctx, 0, scanMatch, 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		if strings.HasSuffix(key, ":wait") {
			fullBase := strings.TrimSuffix(key, ":wait")
			if idx := strings.LastIndex(fullBase, ":g:"); idx != -1 {
				queues = append(queues, fullBase[:idx])
				continue
			}
			queues = append(queues, fullBase)
		}
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	uniqueQueues := make(map[string]bool)
	var cleanQueues []string
	for _, q := range queues {
		cleaned := cleanQueueName(q)
		if !uniqueQueues[cleaned] {
			uniqueQueues[cleaned] = true
			cleanQueues = append(cleanQueues, cleaned)
		}
	}

	return cleanQueues, nil
}

func formatQueueName(queue string) string {
	base := queue
	if strings.HasSuffix(queue, ":meta") {
		base = strings.TrimSuffix(queue, ":meta")
	}
	if !strings.HasPrefix(base, "{") && !strings.HasSuffix(base, "}") {
		return "{" + base + "}"
	}
	return base
}

func cleanQueueName(queue string) string {
	cleaned := strings.TrimPrefix(queue, "{")
	cleaned = strings.TrimSuffix(cleaned, "}")
	return cleaned
}

type Job struct {
	ID        string
	State     string
	Payload   string
	LastError string
	UpdateAt  int64
}

func (m *Monitor) GetJobs(ctx context.Context, queue string, status string, count int) ([]Job, error) {
	base := formatQueueName(queue)
	var ids []string
	var err error

	switch status {
	case "waiting":
		keys, _ := m.client.rdb.Keys(ctx, base+"*:wait").Result()
		for _, key := range keys {
			count = count - len(ids)
			keys, err := m.client.rdb.LRange(ctx, key, 0, int64(count-1)).Result()
			if err != nil {
				continue
			}
			ids = append(ids, keys...)
			if len(ids) >= count {
				ids = ids[:count]
				break
			}
		}
	case "active":
		ids, err = m.client.rdb.ZRange(ctx, base+":active", 0, int64(count-1)).Result()
	case "delayed":
		ids, err = m.client.rdb.ZRange(ctx, base+":delayed", 0, int64(count-1)).Result()
	case "completed":
		ids, err = m.client.rdb.LRange(ctx, base+":completed", 0, int64(count-1)).Result()
	case "failed":
		ids, err = m.client.rdb.LRange(ctx, base+":failed", 0, int64(count-1)).Result()
	default:
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	if len(ids) == 0 {
		return []Job{}, nil
	}

	pipe := m.client.rdb.Pipeline()
	jobCmds := make([]*redis.MapStringStringCmd, len(ids))

	for i, id := range ids {
		jobCmds[i] = pipe.HGetAll(ctx, base+":job:"+id)
	}

	_, err = pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, err
	}

	var jobs []Job
	for i, cmd := range jobCmds {
		val, err := cmd.Result()
		if err != nil {
			continue
		}
		if len(val) == 0 {
			continue
		}

		jobs = append(jobs, Job{
			ID:        ids[i],
			State:     val["state"],
			Payload:   val["payload"],
			LastError: val["last_error"],
		})
	}
	return jobs, nil
}

func (m *Monitor) RetryJob(ctx context.Context, queue, jobID string) error {
	return m.client.OmniqClient.RetryFailed(queue, jobID)
}

func (m *Monitor) RetryFailedBatch(ctx context.Context, queue string, jobIDs []string) (int, error) {
	if len(jobIDs) == 0 {
		return 0, nil
	}
	res, err := m.client.OmniqClient.RetryFailedBatch(queue, jobIDs)
	if err != nil {
		return 0, err
	}
	return len(res), nil
}

func (m *Monitor) RemoveJob(ctx context.Context, queue, jobID, lane string) error {
	_, err := m.client.OmniqClient.RemoveJob(queue, jobID, lane)
	return err
}

func (m *Monitor) RemoveJobsBatch(ctx context.Context, queue, status string, jobIDs []string) (int, error) {
	if len(jobIDs) == 0 {
		return 0, nil
	}
	lane := status
	if lane == "waiting" {
		lane = "wait"
	}

	res, err := m.client.OmniqClient.RemoveJobsBatch(queue, lane, jobIDs)
	if err != nil {
		return 0, err
	}
	return len(res), nil
}
