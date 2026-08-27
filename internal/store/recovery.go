package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func recoverState(eventsPath, snapshotPath string) (snapshot, error) {
	state := emptySnapshot()
	if data, err := os.ReadFile(snapshotPath); err == nil {
		var disk snapshot
		if err := json.Unmarshal(data, &disk); err != nil {
			return state, fmt.Errorf("解析快照: %w", err)
		}
		if disk.SchemaVersion != schemaVersion {
			return state, fmt.Errorf("不支持的快照 schemaVersion %d", disk.SchemaVersion)
		}
		for code, release := range disk.Releases {
			if release.VerificationCode != code {
				return state, fmt.Errorf("快照中的发布凭据索引不一致")
			}
			if err := release.ValidateManifest(); err != nil {
				return state, fmt.Errorf("快照中的发布清单无效: %w", err)
			}
		}
	} else if !os.IsNotExist(err) {
		return state, fmt.Errorf("读取快照: %w", err)
	}

	file, err := os.Open(eventsPath)
	if os.IsNotExist(err) {
		return state, nil
	}
	if err != nil {
		return state, fmt.Errorf("打开事件日志: %w", err)
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	var expected int64 = 1
	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 {
			var event eventRecord
			if err := json.Unmarshal(line, &event); err != nil {
				return state, fmt.Errorf("解析事件 %d: %w", expected, err)
			}
			if event.SchemaVersion != schemaVersion {
				return state, fmt.Errorf("事件 %d schemaVersion 不受支持", expected)
			}
			if event.Sequence != expected {
				return state, fmt.Errorf("事件序号不连续: 期望 %d，实际 %d", expected, event.Sequence)
			}
			if event.Aggregate == nil || event.Aggregate.Survey.ID != event.SurveyID {
				return state, fmt.Errorf("事件 %d 聚合记录不完整", event.Sequence)
			}
			if event.Aggregate.Survey.ExpectedVersion != event.SurveyVersion {
				return state, fmt.Errorf("事件 %d 版本记录不一致", event.Sequence)
			}
			if err := verifyChecksum(event); err != nil {
				return state, err
			}
			for _, release := range event.Aggregate.Releases {
				if err := release.ValidateManifest(); err != nil {
					return state, fmt.Errorf("事件 %d 发布清单无效: %w", event.Sequence, err)
				}
			}
			state.Surveys[event.SurveyID] = event.Aggregate
			state.Idempotency[event.IdempotencyRef] = cloneRaw(event.Result)
			for _, release := range event.Aggregate.Releases {
				state.Releases[release.VerificationCode] = release
			}
			state.LastSequence = event.Sequence
			expected++
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return state, fmt.Errorf("读取事件日志: %w", readErr)
		}
	}
	return state, nil
}
