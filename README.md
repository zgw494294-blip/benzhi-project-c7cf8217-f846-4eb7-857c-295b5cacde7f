# 古籍数字校勘入藏台

古籍数字校勘入藏台面向古籍数字化团队，将建卷、扫描页登记与排序、逐页转录、疑难字处理、完整性检查、版本冻结和最终入藏串成一条可追溯流程。Go 服务同时提供原生单页工作台、同源 JSON HTTP API、SQLite 持久化、按 `sha256` 校验的图像对象和只追加审计轨迹。

## 构建

```text
go build ./cmd/server
```

## 运行

默认监听 `127.0.0.1:19081`，数据保存在当前目录的 `collation.db`：

```text
go run ./cmd/server
```

可通过 `-addr` 指定监听地址，通过 `-data` 指定 SQLite 数据源：

```text
go run ./cmd/server -addr=127.0.0.1:19120 -data=./data/collation.db
```

未传 `-addr` 时也可设置 `PORT`，服务将监听 `127.0.0.1:<PORT>`。浏览器访问服务根路径即可使用工作台。

## 测试与自检

运行全部回归测试：

```text
go test ./...
```

运行真实回环 HTTP 端到端自检；该命令使用内存 SQLite，完成建卷到入藏的完整流程后主动退出：

```text
go run ./cmd/server -selfcheck -addr=127.0.0.1:19081 -data=:memory:
```

所有修改请求都需要携带 `Idempotency-Key`，并在请求体中提交当前 `expectedVersion`。版本过期时 API 返回 `409 Conflict`，冻结后普通编辑会被拒绝。
