#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
base_url="${MOONBOOK_E2E_BASE_URL:-}"
username="${MOONBOOK_E2E_ADMIN_USERNAME:-}"
password="${MOONBOOK_E2E_ADMIN_PASSWORD:-}"
playwright_version="0.1.18"

[[ -n "$base_url" && -n "$username" && -n "$password" ]] || {
  echo "MOONBOOK_E2E_BASE_URL, MOONBOOK_E2E_ADMIN_USERNAME and MOONBOOK_E2E_ADMIN_PASSWORD are required" >&2
  exit 2
}
command -v npx >/dev/null 2>&1 || { echo "missing required command: npx" >&2; exit 2; }
node -e '
  const parsed = new URL(process.argv[1])
  if (!["http:", "https:"].includes(parsed.protocol)) throw new Error("E2E URL must use HTTP(S)")
  if (!["127.0.0.1", "::1", "localhost"].includes(parsed.hostname)) throw new Error("E2E URL must use a loopback host")
' "$base_url"

artifact_base="${MOONBOOK_E2E_OUTPUT_DIR:-$root_dir/output/playwright/management-e2e}"
mkdir -p "$artifact_base"
artifact_dir="$(cd "$artifact_base" && pwd -P)"
session="moonbook-management-$$"
pwcli=(npx --yes --package "@playwright/cli@$playwright_version" playwright-cli --session "$session")

cleanup() { "${pwcli[@]}" close >/dev/null 2>&1 || true; }
trap cleanup EXIT INT TERM

export MOONBOOK_E2E_BASE_URL="$base_url"
export MOONBOOK_E2E_ADMIN_USERNAME="$username"
export MOONBOOK_E2E_ADMIN_PASSWORD="$password"
base_url_json="$(node -e 'process.stdout.write(JSON.stringify(process.argv[1]))' "$base_url")"
username_json="$(node -e 'process.stdout.write(JSON.stringify(process.argv[1]))' "$username")"
password_json="$(node -e 'process.stdout.write(JSON.stringify(process.argv[1]))' "$password")"

pushd "$artifact_dir" >/dev/null
"${pwcli[@]}" open "$base_url"
"${pwcli[@]}" snapshot
e2e_template='async page => {
  const baseUrl = %s
  const username = %s
  const password = %s
  const errors = []
  page.on("console", message => { if (message.type() === "error") errors.push(message.text()) })
  page.on("pageerror", error => errors.push(error.message))
  await page.goto(baseUrl)
  await page.getByPlaceholder("请输入用户名").fill(username)
  await page.getByPlaceholder("请输入密码").fill(password)
  await page.getByRole("button", {name: /登\s*录/}).click()
  await page.waitForFunction(() => !location.hash.includes("/login"), null, {timeout: 15000})
  await page.getByRole("button", {name: "小说管理", exact: true}).click()
  await page.getByText("平台任务", {exact: true}).last().click()
  await page.getByRole("heading", {name: "平台任务", exact: true}).waitFor({timeout: 15000})
  await page.reload()
  await page.getByRole("heading", {name: "平台任务", exact: true}).waitFor({timeout: 15000})
  const response = await page.request.get(baseUrl + "/api/health/live")
  const headers = response.headers()
  for (const [name, expected] of Object.entries({
    "x-content-type-options": "nosniff",
    "x-frame-options": "SAMEORIGIN",
    "referrer-policy": "strict-origin-when-cross-origin"
  })) {
    if (headers[name] !== expected) throw new Error("security header " + name + "=" + headers[name])
  }
  await page.setViewportSize({width: 390, height: 844})
  const overflow = await page.evaluate(() => ({
    width: innerWidth,
    documentWidth: document.documentElement.scrollWidth,
    bodyWidth: document.body.scrollWidth
  }))
  if (overflow.documentWidth > overflow.width || overflow.bodyWidth > overflow.width) {
    throw new Error("mobile overflow " + JSON.stringify(overflow))
  }
  if (errors.length) throw new Error("browser errors: " + errors.join(" | "))
}'
printf -v e2e_code "$e2e_template" "$base_url_json" "$username_json" "$password_json"
"${pwcli[@]}" run-code "$e2e_code" 2>&1 | node -e '
  let output = ""
  process.stdin.setEncoding("utf8")
  process.stdin.on("data", chunk => { output += chunk })
  process.stdin.on("end", () => {
    for (const value of process.argv.slice(1)) {
      for (const variant of [value, JSON.stringify(value).slice(1, -1)]) {
        output = output.split(variant).join("<redacted>")
      }
    }
    process.stdout.write(output)
  })
' "$username" "$password"
"${pwcli[@]}" snapshot
"${pwcli[@]}" screenshot
popd >/dev/null

echo "management E2E passed: login, dynamic menu, refresh, mobile overflow, console, and security headers"
