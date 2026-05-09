# Makefile for AegisGuard Gates Testing

.PHONY: test test-message test-action test-return test-all test-integration test-benchmark demo demo-all help

help:
	@echo "=== AegisGuard 三级策略闸门测试命令 ==="
	@echo ""
	@echo "测试命令:"
	@echo "  make test-message     - 测试消息门控"
	@echo "  make test-action      - 测试动作门控"
	@echo "  make test-return      - 测试返回门控"
	@echo "  make test-all         - 运行所有单元测试"
	@echo "  make test-integration - 运行集成流程测试"
	@echo "  make test-benchmark   - 运行性能基准测试"
	@echo ""
	@echo "演示命令:"
	@echo "  make demo             - 运行完整演示"
	@echo "  make demo-message     - 仅演示消息门控"
	@echo "  make demo-action      - 仅演示动作门控"
	@echo "  make demo-return      - 仅演示返回门控"
	@echo ""
	@echo "实用命令:"
	@echo "  make clean            - 清理编译产物"
	@echo "  make build-demo       - 编译演示程序"
	@echo "  make verbose          - 启用详细输出运行演示"

test-message:
	@echo "运行消息门控测试..."
	cd backend && go test -v ./internal/gates -run TestMessageGatePromptInjection

test-action:
	@echo "运行动作门控测试..."
	cd backend && go test -v ./internal/gates -run TestActionGateToolValidation

test-return:
	@echo "运行返回门控测试..."
	cd backend && go test -v ./internal/gates -run TestReturnGatePIIFiltering

test-all:
	@echo "运行所有门控单元测试..."
	cd backend && go test -v ./internal/gates -run "TestMessageGate|TestActionGate|TestReturnGate|TestPolicyEngine|TestDecisionStore"

test-integration:
	@echo "运行集成流程测试..."
	cd backend && go test -v ./internal/gates -run TestGatesIntegrationFlow

test-benchmark:
	@echo "运行性能基准测试..."
	cd backend && go test -bench=. -benchmem ./internal/gates

demo:
	@echo "运行完整演示程序..."
	cd backend && go run ./cmd/gates-demo/main.go

demo-message:
	@echo "运行消息门控演示..."
	cd backend && go run ./cmd/gates-demo/main.go -action message

demo-action:
	@echo "运行动作门控演示..."
	cd backend && go run ./cmd/gates-demo/main.go -action action

demo-return:
	@echo "运行返回门控演示..."
	cd backend && go run ./cmd/gates-demo/main.go -action return

verbose:
	@echo "运行带详细输出的演示..."
	cd backend && go run ./cmd/gates-demo/main.go -verbose

build-demo:
	@echo "编译演示程序..."
	cd backend/cmd/gates-demo && go build -o gates-demo

clean:
	@echo "清理编译产物..."
	rm -f backend/cmd/gates-demo/gates-demo
	cd backend && go clean ./...

# 综合测试命令
test: test-all test-integration test-benchmark
	@echo ""
	@echo "✓ 所有测试完成！"
