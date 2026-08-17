// Command goldbar is the gold-store trade-in batch settlement CLI.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"goldbar/internal/config"
	"goldbar/internal/input"
	"goldbar/internal/model"
	"goldbar/internal/report"
	"goldbar/internal/settle"
)

const usage = `goldbar — 黄金门店「以旧换新」批量结算工具

用法:
  goldbar settle   -config <file> -input <file|-> -outdir <dir> [-workers N]
  goldbar validate -config <file> -input <file|-> [-workers N]
  goldbar -help

子命令:
  settle     批量结算：写出分户明细 detail.csv、门店日结汇总 summary.json，
             异常行报告 errors.csv（仅在有异常时生成）
  validate   仅校验与试算，不写出任何文件

输入:
  -config   门店参数配置文件 (JSON)：当日金价、成色折价规则、工艺费等
  -input    收银系统导出的换购记录 (CSV)，"-" 表示从标准输入读取
  -outdir   输出目录（仅 settle）
  -workers  并发计算 worker 数，0 表示自动按 CPU 核数

退出码:
  0  全部订单结算成功
  2  部分订单异常（已写出有效明细与异常报告）
  1  整批失败（配置/输入/金价锁定/输出错误或被中断）
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stdout, usage)
		return 0
	}
	switch args[0] {
	case "settle":
		return runSettle(args[1:], stdin, stdout, stderr)
	case "validate":
		return runValidate(args[1:], stdin, stdout, stderr)
	case "-h", "-help", "--help", "help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "未知子命令: %s\n\n%s", args[0], usage)
		return 1
	}
}

func runSettle(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("settle", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfgPath := fs.String("config", "", "门店参数配置文件 (JSON)")
	inPath := fs.String("input", "", "换购记录文件 (CSV)，- 表示标准输入")
	outdir := fs.String("outdir", "", "输出目录")
	workers := fs.Int("workers", 0, "并发计算 worker 数 (0=自动)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *cfgPath == "" || *inPath == "" || *outdir == "" {
		fmt.Fprintln(stderr, "settle 需要 -config -input -outdir 三个参数")
		return 1
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(stderr, "配置错误: %v\n", err)
		return 1
	}
	orders, err := readOrders(*inPath, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "输入错误: %v\n", err)
		return 1
	}
	w := *workers
	if w <= 0 {
		w = runtime.NumCPU()
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	result, err := settle.Run(ctx, cfg, orders, w)
	if err != nil {
		fmt.Fprintf(stderr, "整批失败: %v\n", err)
		return 1
	}
	if err := report.WriteAll(*outdir, result); err != nil {
		fmt.Fprintf(stderr, "输出失败: %v\n", err)
		return 1
	}
	printSummary(stdout, result.Summary)
	if len(result.Errors) > 0 {
		fmt.Fprintf(stdout, "异常 %d 行，详见 %s\n", len(result.Errors), report.ErrorFile)
		return 2
	}
	return 0
}

func runValidate(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfgPath := fs.String("config", "", "门店参数配置文件 (JSON)")
	inPath := fs.String("input", "", "换购记录文件 (CSV)，- 表示标准输入")
	workers := fs.Int("workers", 0, "并发计算 worker 数 (0=自动)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *cfgPath == "" || *inPath == "" {
		fmt.Fprintln(stderr, "validate 需要 -config -input 两个参数")
		return 1
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(stderr, "配置错误: %v\n", err)
		return 1
	}
	orders, err := readOrders(*inPath, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "输入错误: %v\n", err)
		return 1
	}
	w := *workers
	if w <= 0 {
		w = runtime.NumCPU()
	}
	result, err := settle.Run(context.Background(), cfg, orders, w)
	if err != nil {
		fmt.Fprintf(stderr, "整批失败: %v\n", err)
		return 1
	}
	printSummary(stdout, result.Summary)
	if len(result.Errors) > 0 {
		for _, e := range result.Errors {
			fmt.Fprintf(stdout, "异常 第%d行 %s [%s]: %s\n", e.LineNumber, e.OrderID, e.Code, e.Message)
		}
		return 2
	}
	return 0
}

func readOrders(path string, stdin io.Reader) ([]model.Order, error) {
	if path == "-" {
		return input.Parse(stdin)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开输入文件 %q 失败: %w", path, err)
	}
	defer f.Close()
	return input.Parse(f)
}

func printSummary(w io.Writer, s model.Summary) {
	fmt.Fprintf(w, "门店 %s %s | 日期 %s | 金价 %.2f %s\n", s.StoreID, s.StoreName, s.TradeDate, s.GoldPrice, s.Currency)
	fmt.Fprintf(w, "订单 %d (有效 %d / 异常 %d)\n", s.TotalOrders, s.ValidOrders, s.ErrorOrders)
	fmt.Fprintf(w, "旧金折现合计 %.2f | 新品售价合计 %.2f | 工艺费合计 %.2f\n", s.TotalOldDiscount, s.TotalNewSalePrice, s.TotalCraftFee)
	fmt.Fprintf(w, "应补差价合计 %.2f | 应退金额合计 %.2f | 门店应收净额 %.2f\n", s.TotalPayable, s.TotalRefund, s.NetStoreReceivable)
}
