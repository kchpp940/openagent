#!/usr/bin/env bash
# check_config_drift.sh — detect scattered config usage outside ConfigRegistry.
#
# This script catches NEW config reads that bypass the unified ConfigRegistry.
#
# Whitelist: annotate any line that legitimately bypasses ConfigRegistry with:
#   // config-registry:allow   (Go)
#   // config-registry:allow   (JS/JSX)
#
# Key design:
#   - conf.GetConfigString/Bool/Int in conf/ package = OK (delegates to Registry)
#   - conf.GetConfigString/Bool/Int outside conf/ = OK (calls unified API)
#   - conf.GetConfigString2/Bool2/Int2 anywhere = OK (Registry API itself)
#   - Direct beego.AppConfig / os.Getenv outside conf/ = VIOLATION (bypasses Registry)
#   - process.env / REACT_APP_ in frontend outside serviceWorker = VIOLATION
#
# Exit 0 if clean, exit 1 if violations found.

set -euo pipefail
cd "$(git rev-parse --show-toplevel 2>/dev/null || echo .)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

VIOLATIONS=0

# ────────────────────────────────────────────────────
# 1. Go: beego.AppConfig direct reads outside conf/
#    These bypass ConfigRegistry and should use conf.GetConfigString instead.
# ────────────────────────────────────────────────────
echo "=== Go: beego.AppConfig direct reads outside conf/ ==="
beego_matches=$(grep -rn --include='*.go' \
  -E 'beego\.AppConfig\.(String|Bool|Int|Int64|Float|DefaultString|DefaultBool|DefaultInt|DefaultInt64|DefaultFloat)\(' . 2>/dev/null \
  | grep -v 'config-registry:allow' \
  | grep -v '^./conf/' || true)
if [ -n "$beego_matches" ]; then
  echo -e "${RED}FAIL${NC} [go-beego-appconfig]"
  echo "$beego_matches"
  echo ""
  VIOLATIONS=$((VIOLATIONS + $(echo "$beego_matches" | wc -l | tr -d ' ')))
else
  echo -e "${GREEN}PASS${NC} [go-beego-appconfig]"
fi

# ────────────────────────────────────────────────────
# 2. Go: os.Getenv / os.LookupEnv outside conf/config_registry.go
#    Config keys must go through ConfigRegistry.resolveValue().
#    System env vars (paths, tokens) need a // config-registry:allow comment.
# ────────────────────────────────────────────────────
echo ""
echo "=== Go: os.Getenv / os.LookupEnv outside conf/config_registry.go ==="
env_matches=$(grep -rn --include='*.go' \
  -E 'os\.(Getenv|LookupEnv)\(' . 2>/dev/null \
  | grep -v 'config-registry:allow' \
  | grep -v '^./conf/config_registry\.go' || true)
if [ -n "$env_matches" ]; then
  echo -e "${RED}FAIL${NC} [go-os-env]"
  echo "$env_matches"
  echo ""
  VIOLATIONS=$((VIOLATIONS + $(echo "$env_matches" | wc -l | tr -d ' ')))
else
  echo -e "${GREEN}PASS${NC} [go-os-env]"
fi

# ────────────────────────────────────────────────────
# 3. Go: conf.GetConfigString/Bool/Int outside conf/ package
#    These are LEGITIMATE — they delegate to ConfigRegistry via conf.go.
#    But we scan for NEW GetConfigString2/Bool2/Int2 calls outside conf/
#    which should not be used directly (use the non-2 versions instead).
# ────────────────────────────────────────────────────
echo ""
echo "=== Go: direct GetConfigString2/Bool2/Int2 calls outside conf/ ==="
direct2_matches=$(grep -rn --include='*.go' \
  -E 'GetConfig(String2|Bool2|Int2)\(' . 2>/dev/null \
  | grep -v 'config-registry:allow' \
  | grep -v '^./conf/' || true)
if [ -n "$direct2_matches" ]; then
  echo -e "${RED}FAIL${NC} [go-direct-registry-api]"
  echo "$direct2_matches"
  echo ""
  VIOLATIONS=$((VIOLATIONS + $(echo "$direct2_matches" | wc -l | tr -d ' ')))
else
  echo -e "${GREEN}PASS${NC} [go-direct-registry-api]"
fi

# ────────────────────────────────────────────────────
# 4. JS: process.env outside serviceWorker.js
# ────────────────────────────────────────────────────
echo ""
echo "=== JS: process.env outside serviceWorker.js ==="
js_env_matches=$(grep -rn --include='*.js' --include='*.jsx' \
  -E 'process\.env\.' web/src/ 2>/dev/null \
  | grep -v 'config-registry:allow' \
  | grep -v 'serviceWorker\.js' || true)
if [ -n "$js_env_matches" ]; then
  echo -e "${RED}FAIL${NC} [js-process-env]"
  echo "$js_env_matches"
  echo ""
  VIOLATIONS=$((VIOLATIONS + $(echo "$js_env_matches" | wc -l | tr -d ' ')))
else
  echo -e "${GREEN}PASS${NC} [js-process-env]"
fi

# ────────────────────────────────────────────────────
# 5. JS: REACT_APP_ env vars
# ────────────────────────────────────────────────────
echo ""
echo "=== JS: REACT_APP_ env vars ==="
react_app_matches=$(grep -rn --include='*.js' --include='*.jsx' \
  -E 'REACT_APP_' web/src/ 2>/dev/null \
  | grep -v 'config-registry:allow' || true)
if [ -n "$react_app_matches" ]; then
  echo -e "${RED}FAIL${NC} [js-react-app-env]"
  echo "$react_app_matches"
  echo ""
  VIOLATIONS=$((VIOLATIONS + $(echo "$react_app_matches" | wc -l | tr -d ' ')))
else
  echo -e "${GREEN}PASS${NC} [js-react-app-env]"
fi

# ────────────────────────────────────────────────────
# 6. JS: hardcoded config default values in frontend
#    Any non-whitelisted config key string with a value = possible drift.
#    Strict: any match fails the build; add // config-registry:allow to exempt.
# ────────────────────────────────────────────────────
echo ""
echo "=== JS: hardcoded config default values in frontend ==="
js_default_matches=$(grep -rn --include='*.js' --include='*.jsx' \
  -E '(htmlTitle|themeColor|staticBaseUrl|faviconUrl|logoUrl|navbarHtml|footerHtml|checkUserBalance|ipParsingMode|parentDbName|hubDbNames|socks5Proxy|logConfig|casdoorEndpoint|casdoorOrganization|casdoorApplication|defaultColorPrimary|batchSize)\s*[=:]\s*["\x27][^"\x27]*["\x27]' web/src/ 2>/dev/null \
  | grep -v 'config-registry:allow' \
  | grep -v 'Conf\.js' \
  | grep -v 'ThemeSetting\.js' \
  | grep -v 'SiteEditPage\.js' \
  | grep -v 'SiteListPage\.js' \
  | grep -v 'StoreListPage\.js' \
  | grep -v 'App\.js' || true)
if [ -n "$js_default_matches" ]; then
  echo -e "${RED}FAIL${NC} [js-config-defaults]"
  echo "$js_default_matches"
  echo ""
  VIOLATIONS=$((VIOLATIONS + $(echo "$js_default_matches" | wc -l | tr -d ' ')))
else
  echo -e "${GREEN}PASS${NC} [js-config-defaults]"
fi

# ────────────────────────────────────────────────────
# 7. Go: sensitive field check — verify GetAllConfigMetadata masks
# ────────────────────────────────────────────────────
echo ""
echo "=== Go: sensitive field masking in GetAllConfigMetadata ==="
if grep -q '!item.Sensitive' conf/config_registry.go && \
   grep -A2 '!item.Sensitive' conf/config_registry.go | grep -q 'meta.CurrentValue'; then
  echo -e "${GREEN}PASS${NC} [sensitive-masking] Sensitive fields skip CurrentValue"
else
  echo -e "${RED}FAIL${NC} [sensitive-masking] Cannot confirm sensitive field masking"
  VIOLATIONS=$((VIOLATIONS + 1))
fi

# ────────────────────────────────────────────────────
# 8. Go: SiteField whitelist in BuildSiteOverridesFromConfigMap
# ────────────────────────────────────────────────────
echo ""
echo "=== Go: SiteField whitelist guard in BuildSiteOverridesFromConfigMap ==="
if grep -q 'SiteField == ""' conf/config_registry.go; then
  echo -e "${GREEN}PASS${NC} [sitefield-whitelist] Unregistered SiteField keys are skipped"
else
  echo -e "${RED}FAIL${NC} [sitefield-whitelist] Cannot confirm SiteField whitelist"
  VIOLATIONS=$((VIOLATIONS + 1))
fi

# ────────────────────────────────────────────────────
# Summary
# ────────────────────────────────────────────────────
echo ""
echo "========================================"
if [ "$VIOLATIONS" -eq 0 ]; then
  echo -e "${GREEN}All checks passed.${NC}"
  exit 0
else
  echo -e "${RED}$VIOLATIONS violation(s) found.${NC}"
  echo "Add '// config-registry:allow' comment on the line to whitelist."
  exit 1
fi
