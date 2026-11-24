# 接入 Home Assistant

TrackFerry 可以把每台音箱的真实状态实时给 Home Assistant：在放什么、放到哪、歌词、封面、主色、谁说了什么口令，并接受播放控制。默认关闭。

## 打开

1. “设置 → Home Assistant”打开开关，点“生成令牌”。令牌只显示一次。
2. 在 Home Assistant 的集成里填服务地址（与管理页相同，默认 `http://<NAS 地址>:18561`）和令牌。

令牌只能用于 `/api/ha/` 下的接口，不能登录管理页。换新令牌或关闭开关时，已有连接立即断开。关闭时这些接口一律返回 404。

只有状态为“观察”或“接管”的音箱会给出状态、接受控制；“关闭”的音箱只列出名字，`state` 为 `off`。

## 接口

请求都带 `Authorization: Bearer <令牌>`。

| 接口 | 说明 |
| --- | --- |
| `GET /api/ha/state` | 一次性全量 |
| `GET /api/ha/events` | 实时推送（SSE） |
| `GET /api/ha/cover/{id}?size=160\|600\|1024` | 封面，默认 600。实际地址取 `cover` 字段 |
| `GET /api/ha/lyrics/{track_id}` | 歌词。实际地址取 `lyrics` 字段 |
| `POST /api/ha/speakers/{speaker_id}/control` | 控制 |

状态码：401 令牌不对；404 开关没开，或没有这个对象；错误体为 `{"error": "代号", "message": "说明"}`。

### 实时推送

- 连上后第一条总是 `snapshot`（全量），之后是 `speaker`（一台音箱的全量）。音箱增减时再发一次 `snapshot`。断线后直接重连即可，不需要补发。
- 每 20 秒一行 `: ` 心跳。45 秒没有任何数据就应重连，建议间隔 1、2、5、10、30 秒。
- `id` 为 `<instance_id>-<seq>`。`instance_id` 变了表示服务重启过，`seq` 重新计数。
- 只在有意义的变化时推送：状态、歌曲、音量、口令，或进度和预期差了 1.5 秒以上。正常播放不逐秒推送。

可以重放的完整样本：[samples/ha-events.sse](samples/ha-events.sse)（数据是虚构的），依次是空闲、起播、播放、和小爱说话、续播、切到线上歌、放完、小爱自己播放。

### 一台音箱

```json
{
  "schema_version": 1,
  "server_time": "2026-11-24T12:00:08.000Z",
  "instance_id": "a1b2c3d4e5f6",
  "seq": 3,
  "speakers": [
    {
      "speaker_id": "示例-1",
      "miot_did": "100000001",
      "name": "客厅音箱",
      "model": "LX06",
      "mode": "takeover",
      "online": true,
      "controls": [
        "play",
        "pause",
        "stop",
        "volume",
        "wake"
      ],
      "state": "playing",
      "activity": null,
      "source_kind": "trackferry",
      "source": "library",
      "play_id": 101,
      "track_id": 7,
      "title": "示例歌曲甲",
      "artist": "示例歌手",
      "album": "示例专辑",
      "duration_ms": 200000,
      "position_ms": 900,
      "position_updated_at": "2026-11-24T12:00:08.000Z",
      "playback_rate": 1,
      "volume": 35,
      "muted": false,
      "cover": "/api/ha/cover/c1",
      "cover_version": "c1",
      "cover_colors": [
        "#c81e1e",
        "#143cb4"
      ],
      "lyrics": "/api/ha/lyrics/7",
      "last_request": {
        "text": "播放示例歌曲甲",
        "at": "2026-11-24T12:00:05Z"
      },
      "updated_at": "2026-11-24T12:00:08.000Z",
      "stale": false
    }
  ]
}
```

| 字段 | 说明 |
| --- | --- |
| `speaker_id` · `miot_did` | TrackFerry 的编号，稳定不变；米家的 did，可用来和小米集成对应 |
| `model` · `mode` · `online` | 型号；`takeover`、`observe`、`off`；上次同步时是否在线 |
| `controls` | 这台音箱能用的操作 |
| `state` | `playing`、`paused`、`idle`、`off` |
| `activity` | `starting` 正在起播，`talking` 有人在和小爱说话，`resuming` 正在续播，其余为 null |
| `source_kind` | `trackferry` 我们放的，`xiaoai` 小爱自己放的，`unknown` 分不出，没有在放为 null |
| `source` | `library`、`kuwo`、`kugou`、`migu` |
| `play_id` | 一次播放的编号。同一首歌再放一遍也会变 |
| `title` · `artist` · `album` · `track_id` | 只有我们放的才有；小爱自己放的为 null，不猜 |
| `duration_ms` · `position_ms` · `position_updated_at` · `playback_rate` | 当前进度 = `position_ms` + （现在 − `position_updated_at`）× `playback_rate` |
| `volume` · `muted` | 起播时的短暂静音不算，给的是听的人设定的音量 |
| `cover` · `cover_version` · `cover_colors` | 曲库里的歌才有封面；`cover_version` 变了才需要重新取；主色最多 3 个，稍后补发，也可能为空 |
| `lyrics` | 曲库里的歌才有地址，歌词本身可能为空 |
| `last_request` | 这台音箱最近一次点歌或播放控制的口令，`at` 是说话的时间；重启后为 null |
| `stale` | 超过 30 秒没有读到音箱状态 |

以后可能增加字段，请忽略不认识的。字段含义改变时 `schema_version` 加一。

### 歌词

```json
{"schema_version": 1, "track_id": 7, "synced": true, "offset_ms": 0,
 "lines": [{"t": 1000, "text": "第一句"}, {"t": 5200, "text": "第二句"}]}
```

`t` 是毫秒，歌词文件自带的偏移已经算进去。`synced` 为 false 时 `t` 为 null，只有文字。

### 控制

```json
{"action": "volume", "volume": 35}
```

| action | 作用 |
| --- | --- |
| `play` · `pause` · `stop` | 播放、暂停、停止。暂停或停止后，歌不会自己续上 |
| `volume` | 0 到 100。正在短暂静音时，恢复到这个音量 |
| `wake` | 唤醒小爱，等你说话；只有 `controls` 里有 `wake` 的型号支持 |

成功返回 204。400 参数不对，404 没有这台音箱，409 没有接入或型号不支持，502 音箱没有照做。

## 时钟

`position_updated_at` 和 `server_time` 都是 TrackFerry 的 UTC 时间。Home Assistant 的时钟可能不同，收到消息时记下 `server_time` 与本地时间的差再推算进度。

## 限制

- 音箱很少报告进度，我们放的歌用自己的时钟计算，误差通常在 1 秒以内。
- 小爱自己放歌时只知道在不在放，没有歌名。
- 没有节拍和频谱数据。
- 没有人连接时，不会为此去查询音箱；连接时，我们没有在放的音箱每 5 到 10 秒查一次。
