#!/usr/bin/env bash
# MCP/Tool/Skill 统一执行上下文 —— 开发检查脚本
#
# 用途：
#   扫描 Go 源码中是否存在绕过 ToolExecutionContext/ExternalCallResult/SchemaParseResult
#   的违规模式，避免重复的错误包装、参数解析、schema 处理逻辑。
#
# 用法：
#   ./scripts/check-mcp-lint.sh          # 检查 mcp/ object/ model/ tool/ controllers/
#   ./scripts/check-mcp-lint.sh --all    # 检查所有 .go 文件（排除 _test.go）
#
# 白名单标记（违规行上方 3 行内必须包含以下注释之一）：
#   // ---- SDK 原始调用边界 ----
#   // ---- 流式输出边界 ----
#   // ---- 流式输出边界：xxx ----
#   // ---- 工具定义层边界 ----
#   // ---- 工具定义层边界：xxx ----
#   // ---- 纯编解码工具函数边界 ----
#   // ---- 旧 API 兼容层 ----
#   // ---- 数据持久化边界 ----
#   // ---- 数据持久化边界：xxx ----
#   // ---- 模型 API 适配边界 ----
#   // ---- 模型 API 适配边界：xxx ----
#   // ---- 公共模块内部实现 ----
#
# 退出码：
#   0 - 无违规
#   1 - 发现违规
# ---------------------------------------------------------------------------

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

CHECK_ALL=false
for arg in "$@"; do
	case "$arg" in
		--all) CHECK_ALL=true ;;
	esac
done

if $CHECK_ALL; then
	SEARCH_DIRS=("$PROJECT_ROOT")
else
	SEARCH_DIRS=(
		"$PROJECT_ROOT/mcp"
		"$PROJECT_ROOT/object"
		"$PROJECT_ROOT/model"
		"$PROJECT_ROOT/tool"
		"$PROJECT_ROOT/controllers"
	)
fi

# 白名单注释正则（匹配前缀即可）
WHITELIST_PREFIX='// ---- (SDK 原始调用边界|流式输出边界|工具定义层边界|纯编解码工具函数边界|旧 API 兼容层|数据持久化边界|模型 API 适配边界|公共模块内部实现)'

# 违规模式：名称|grep 正则|解释
VIOLATION_PATTERNS=(
	"CALLTOOL_DIRECT|\.CallTool\(|必须通过 mcp\.CallToolWithContext 或 ToolSet\.ExecuteToolWithContext 执行工具调用"
	"LISTTOOLS_DIRECT|\.ListTools\(|必须通过 mcp\.ListToolsWithContext 获取工具列表"
	"UNMARSHAL_ARGS|json\.Unmarshal\([^)]*Arguments|必须通过 mcp\.ParseToolArguments 解析工具参数"
	"UNMARSHAL_TEST|json\.Unmarshal\([^)]*[Tt]est[Cc]ontent|必须通过 mcp\.ParseTestContent 解析测试内容"
	"MARSHAL_CONTENT|json\.Marshal\([^)]*\.Content\)|必须通过 mcp\.CallToolResultToExternalResult 处理工具结果"
	"TOOLCALLERR_CONST|ToolCallErr[A-Z]|必须使用 mcp\.ErrKind* 常量，禁止使用旧的 ToolCallErr* 常量"
	"KIND_MSG_ERROR|fmt\.Sprintf\(\"\\\[%s\\\] %s\"|必须通过 mcp\.NewExternalCallResultError 包装工具错误"
	"RESULT_ISERROR|result\.IsError|必须通过 mcp\.CallToolResultToExternalResult 处理工具结果的错误状态"
)

info()  { printf '\033[1;34m[info]\033[0m %s\n' "$*"; }
error() { printf '\033[1;31m[error]\033[0m %s\n' "$*"; }
pass()  { printf '\033[1;32m[pass]\033[0m %s\n' "$*"; }

info "扫描目录: ${SEARCH_DIRS[*]}"
info ""

# 收集文件
GO_FILES=()
for dir in "${SEARCH_DIRS[@]}"; do
	while IFS= read -r -d '' f; do
		if [[ "$f" != *"_test.go" ]]; then
			GO_FILES+=("$f")
		fi
	done < <(find "$dir" -type f -name "*.go" -print0)
done

info "共找到 ${#GO_FILES[@]} 个 Go 文件（排除 _test.go）"
info ""

TOTAL_VIOLATIONS=0
PASS=true

# 检查单个违规模式
check_pattern() {
	local pname="$1"
	local pregex="$2"
	local pdesc="$3"

	# 用 grep 找到所有命中（带行号）
	while IFS= read -r match; do
		[[ -z "$match" ]] && continue

		local file="${match%%:*}"
		local lineno="${match#*:}"
		lineno="${lineno%%:*}"
		local rel_path="${file#$PROJECT_ROOT/}"
		local line_content="${match#*:*:}"

		# 跳过 mcp/execution.go 的注释示例（文件头的反模式列表）
		if [[ "$rel_path" == "mcp/execution.go" && "$lineno" -le 70 ]]; then
			continue
		fi

		# 检查违规行上方 3 行内是否有白名单注释
		local has_whitelist=false
		local start_line=$((lineno > 3 ? lineno - 3 : 1))
		local context_lines
		context_lines="$(sed -n "${start_line},${lineno}p" "$file")"
		if echo "$context_lines" | grep -qE "$WHITELIST_PREFIX"; then
			has_whitelist=true
		fi

		# 特殊：函数定义上方的注释（可能在更远的地方）
		if ! $has_whitelist; then
			# 检查当前行是否在公共模块内部实现函数内
			if [[ "$rel_path" == "mcp/execution.go" ]]; then
				if echo "$context_lines" | grep -qE 'func (CallToolResultToExternalResult|ParseTestContent|ParseToolArguments|ParseInputSchema)'; then
					has_whitelist=true
				fi
			fi
		fi

		if ! $has_whitelist; then
			TOTAL_VIOLATIONS=$((TOTAL_VIOLATIONS + 1))
			PASS=false
			error "违规 [$pname] $rel_path:$lineno"
			echo "  行: $line_content" | sed 's/^/         /'
			echo "  原因: $pdesc" | sed 's/^/         /'
			echo "  修复: 参考 mcp/execution.go 文件头的开发规范，或在该行上方 3 行内添加白名单注释" | sed 's/^/         /'
			echo ""
		fi
	done < <(grep -nHE "$pregex" "${GO_FILES[@]}" 2>/dev/null || true)
}

# 扫描每个违规模式
for pattern in "${VIOLATION_PATTERNS[@]}"; do
	IFS='|' read -r pname pregex pdesc <<< "$pattern"
	check_pattern "$pname" "$pregex" "$pdesc"
done

# 输出结果
info "=========================================================================="
if $PASS; then
	pass "✅ MCP 规范检查通过：$TOTAL_VIOLATIONS 个违规"
	exit 0
else
	error "❌ MCP 规范检查失败：发现 $TOTAL_VIOLATIONS 个违规"
	echo ""
	echo "   参考文档：mcp/execution.go 文件头的使用边界与开发规范"
	echo ""
	exit 1
fi
