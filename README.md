# 野迹裁定台

野迹裁定台是面向野外生态调查员、物种标注复核员和调查数据负责人的红外相机调查服务。它以版本化 JSON HTTP API 串联调查建案、相机点位登记、方案锁定、独立标注核验、争议裁定、定向修正、独立复核、不可变发布和凭据核验。

## 业务约束

- 调查创建后处于 `draft`，至少登记一个点位才能锁定判定方案。
- `draft` 阶段可原子修订标题、负责人、排序去重后的物种目录，也可更新或撤销点位；每次变更都校验版本并记录审核轨迹。
- 方案进入 `protocol_locked` 后，不再允许直接修改点位或物种目录。
- 每条观测必须属于已登记点位，拍摄时间必须位于点位布设区间内。
- 观测批量登记每批支持 1 至 100 条，批内或调查已有证据引用、SHA-256 校验和重复时整批不写入。
- 核验同时检查证据引用、SHA-256 格式的校验和、物种目录以及两份独立标注的一致性。
- `disputed` 观测必须由独立复核员裁定；`correction_required` 观测只能按裁定要求修正后重新核验。
- 仍有未核验观测、证据问题或未决争议时，不能申请复核、批准或发布。
- 批准人不能是调查负责人。一个调查只能生成一个不可变发布版本。
- 发布版本固化按拍摄时间稳定排序的数据清单和批准信息；凭据核验会同时验证凭据索引与清单 SHA-256 摘要。
- 所有写接口要求 `Idempotency-Key` 请求头和当前 `expectedVersion`；建案接口只要求 `Idempotency-Key`。

## 构建、运行与测试

项目只使用 Go 标准库，要求 Go 1.23 或更新版本。

```text
go build ./cmd/trapreview
go test ./...
go run ./cmd/trapreview -addr=127.0.0.1:19081 -data-dir=./data
```

默认监听 `127.0.0.1:19081`。可通过 `-addr=127.0.0.1:<port>` 指定完整地址；若设置 `PORT` 为合法端口号且未覆盖 `-addr`，服务绑定 `127.0.0.1:<PORT>`，不会默认绑定 `0.0.0.0`。

运行真实端到端自检：

```text
go run ./cmd/trapreview -selfcheck -selfcheck-timeout=15s -addr=127.0.0.1:19081
```

自检会启动真实回环 HTTP 服务，在临时数据目录中依次执行建案、点位登记、方案锁定、冲突观测提交、核验、裁定修正、重新核验、复核批准、发布和凭据核验，并在完成后自行关闭服务和删除临时数据。

## HTTP API

所有响应均使用 `application/json`，成功数据位于 `data`，失败信息位于 `error`。公开路由统一使用 `/api/v1` 前缀。

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `POST` | `/api/v1/surveys` | 创建调查任务 |
| `GET` | `/api/v1/surveys/{surveyID}` | 查询调查详情与审核轨迹 |
| `PATCH` / `PUT` | `/api/v1/surveys/{surveyID}` | 在 `draft` 状态修订调查方案 |
| `POST` | `/api/v1/surveys/{surveyID}/stations` | 登记相机点位 |
| `PATCH` / `PUT` | `/api/v1/surveys/{surveyID}/stations/{stationID}` | 在锁定前更新相机点位 |
| `DELETE` | `/api/v1/surveys/{surveyID}/stations/{stationID}` | 在锁定前撤销相机点位 |
| `POST` | `/api/v1/surveys/{surveyID}/lock` | 锁定调查方案 |
| `POST` | `/api/v1/surveys/{surveyID}/observations` | 提交观测和两份标注 |
| `POST` | `/api/v1/surveys/{surveyID}/observations/batch` | 原子批量登记 1 至 100 条观测 |
| `POST` | `/api/v1/surveys/{surveyID}/verify` | 核验 `observationIds` 指定的观测；省略时核验全部待核验观测 |
| `POST` | `/api/v1/surveys/{surveyID}/observations/{observationID}/verify` | 定向核验单条待核验观测 |
| `GET` | `/api/v1/surveys/{surveyID}/verification-results` | 按点位、状态、质量标记分页查询核验结果 |
| `GET` | `/api/v1/surveys/{surveyID}/adjudications/pending` | 查询待裁定队列 |
| `POST` | `/api/v1/surveys/{surveyID}/observations/{observationID}/adjudications` | 裁定争议观测 |
| `POST` | `/api/v1/surveys/{surveyID}/observations/{observationID}/corrections` | 定向修正观测 |
| `POST` | `/api/v1/surveys/{surveyID}/review` | 申请独立复核 |
| `GET` | `/api/v1/surveys/{surveyID}/review-summary` | 查询审核摘要 |
| `POST` | `/api/v1/surveys/{surveyID}/approve` | 批准调查 |
| `POST` | `/api/v1/surveys/{surveyID}/releases` | 生成不可变发布版本 |
| `GET` | `/api/v1/releases/verify/{verificationCode}` | 核验发布凭据 |
| `GET` | `/api/v1/releases/verify/{verificationCode}/contents` | 按物种分页查询不可变发布清单 |

请求体上限为 1 MiB，未知 JSON 字段会被拒绝。版本冲突、非法状态迁移、未决争议和重复发布返回 `409`；字段错误返回 `400`；资源不存在返回 `404`。

## 持久化与恢复

`-data-dir` 中的 `events.jsonl` 是按提交顺序追加并同步到磁盘的事件记录，每条事件包含 schema、连续序号、调查版本、完整聚合记录和校验和。`snapshot.json` 通过临时文件同步和原子替换生成，保存一致读取视图、幂等结果及发布凭据索引。启动时服务从事件日志重建状态，并校验 `schemaVersion`、事件序号、聚合完整性、版本、事件校验和、发布凭据索引及发布清单摘要；损坏的数据会使启动明确失败。
