# goldbar — 黄金门店「以旧换新」批量结算工具

`goldbar` 是一个自包含的 Go 命令行工具，用于黄金零售门店每日批量结算「以旧换新」订单。
店员从收银系统导出当日换购记录 CSV，配合门店参数配置（当日金价、税收新政后的成色折价规则、
工艺费），工具逐行计算旧金折现额、新饰品克重价与工艺费，得出每单应补差价或应退金额，
输出分户明细与门店日结汇总，并以退出码区分全部成功、部分异常与整批失败。

## 业务规则

- **金价锁定**：同一文件内所有订单的金价必须与配置中的当日金价一致，防止跨时段价格漂移；不一致则整批失败。
- **成色折价**：按成色（千分比）选择「不超过该成色的最高折价档位」，
  旧金折现额 = 旧金克重 × 当日金价 × (成色/1000) × 折价率。
- **新品售价**：新饰品克重价 = 新品克重 × 当日金价；工艺费 = 新品克重 × 工艺费率；新品售价 = 克重价 + 工艺费。
- **结算**：由 (新品售价 − 旧金折现额) 的符号决定应补差价（正值）或应退金额（负值）。
- **行级异常**（不写入汇总，按行号报错）：成色低于 900、克重为负、旧金折现额超过新品售价。
- **幂等与失败恢复**：输出确定性（按输入顺序、金额按分取整、无时间戳），重复执行结果一致；
  写入采用「先全部暂存到临时文件，再原子改名」，失败时不残留半成品明细文件，便于中断后安全重跑。

## 安装与启动

需要 Go 1.26：

    go build -trimpath -ldflags="-s -w" -o goldbar ./cmd/goldbar
    ./goldbar -help

## 主要命令

    goldbar settle   -config <file> -input <file|-> -outdir <dir> [-workers N]
    goldbar validate -config <file> -input <file|-> [-workers N]

- `settle`：批量结算，写出 `detail.csv`（分户明细）、`summary.json`（门店日结汇总），
  有异常时额外写出 `errors.csv`。
- `validate`：仅校验与试算，不写出任何文件。
- `-input -` 表示从标准输入读取换购记录。

退出码：

| 退出码 | 含义 |
| ------ | ---- |
| `0` | 全部订单结算成功 |
| `2` | 部分订单异常（已写出有效明细与异常报告） |
| `1` | 整批失败（配置/输入/金价锁定/输出错误或被中断） |

## 输入与输出

门店配置为 JSON（示例见 `examples/config.json`）：

```json
{
  "store_id": "SH001",
  "store_name": "上海旗舰店",
  "trade_date": "2026-08-17",
  "gold_price": 768.50,
  "karat_discount_rules": [
    {"karat": 999, "rate": 0.98},
    {"karat": 990, "rate": 0.95},
    {"karat": 900, "rate": 0.90},
    {"karat": 750, "rate": 0.80}
  ],
  "craft_rate_per_gram": 12.00,
  "currency": "CNY"
}
```

换购记录为 CSV，表头固定（示例见 `examples/records.csv`）：

    order_id,customer,old_karat,old_weight,new_product_code,new_weight,gold_price
    A001,张三,999,2.000,G001,8.000,768.50

输出文件（写入 `-outdir` 目录）：

- `detail.csv`：每单明细（成色、克重、旧金折现额、克重价、工艺费、新品售价、应补/应退、方向）。
- `summary.json`：门店日结汇总（订单数、有效/异常数、各项合计、门店应收净额）。
- `errors.csv`：异常行（行号、订单号、错误码、说明），仅在有异常时生成；无异常时自动清理历史残留。

## 测试

    go fmt ./...
    go mod tidy
    go mod verify
    go list -mod=readonly -m all
    go build ./...
    go test -timeout=120s -count=1 ./...

测试覆盖：正常路径、错误路径、状态汇总、并发确定性、取消、幂等与失败恢复（原子写入不残留）。

## Docker

构建（本机架构）：

    docker build -t goldbar:1.0.0 .

构建并加载指定架构（amd64）：

    docker buildx build --platform linux/amd64 -t goldbar:1.0.0 --load .

多架构构建并推送（amd64 + arm64，需 buildx 与镜像仓库）：

    docker buildx build --platform linux/amd64,linux/arm64 -t goldbar:1.0.0 --push .

运行（挂载配置与记录，输出到当前目录的 `out/`）：

    docker run --rm -v "$PWD/examples:/data" -v "$PWD/out:/out" goldbar:1.0.0 \
      settle -config /data/config.json -input /data/records.csv -outdir /out
