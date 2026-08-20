#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
output_dir="${1:-}"

usage() {
  cat >&2 <<'EOF'
usage: scripts/scan-supply-chain.sh /absolute/existing/output/directory

Required environment variables:
  SERVER_IMAGE   built server image reference
  WEB_IMAGE      built management frontend image reference
  READER_IMAGE   built reader image reference
EOF
}

[[ -n "$output_dir" ]] || { usage; exit 2; }
[[ "$output_dir" == /* ]] || { echo "output directory must be absolute" >&2; exit 2; }
[[ -d "$output_dir" && ! -L "$output_dir" ]] || {
  echo "output directory must already exist and must not be a symlink" >&2
  exit 2
}
output_dir="$(cd "$output_dir" && pwd -P)"
case "$output_dir/" in
  "$root_dir/"*) echo "resolved output directory must be outside the repository" >&2; exit 2 ;;
esac

: "${SERVER_IMAGE:?SERVER_IMAGE must name a built image}"
: "${WEB_IMAGE:?WEB_IMAGE must name a built image}"
: "${READER_IMAGE:?READER_IMAGE must name a built image}"

for command in docker go jq npm pnpm shasum; do
  command -v "$command" >/dev/null 2>&1 || { echo "missing required command: $command" >&2; exit 2; }
done

readonly GOVULNCHECK_VERSION="v1.1.4"
readonly TRIVY_IMAGE="aquasec/trivy:0.67.2@sha256:e2b22eac59c02003d8749f5b8d9bd073b62e30fefaef5b7c8371204e0a4b0c08"
readonly SYFT_IMAGE="anchore/syft:v1.51.0@sha256:678bfa565b60f747aac0f8e964fe5588a24445b8d0a480e91f6efd70020dfbb0"

mkdir -p "$output_dir/reports" "$output_dir/sbom" "$output_dir/cache/trivy" "$output_dir/images"
cleanup() { rm -f "$output_dir"/images/*.tar; }
trap cleanup EXIT

echo "scanning source dependency locks"
pushd "$root_dir/server" >/dev/null
set +e
go run "golang.org/x/vuln/cmd/govulncheck@$GOVULNCHECK_VERSION" -json ./... \
  >"$output_dir/reports/server-govulncheck.json"
govuln_status=$?
set -e
popd >/dev/null

pushd "$root_dir/web" >/dev/null
set +e
pnpm audit --prod --json >"$output_dir/reports/web-pnpm-audit.json"
web_audit_status=$?
set -e
popd >/dev/null

pushd "$root_dir/reader-ui" >/dev/null
set +e
npm audit --omit=dev --json >"$output_dir/reports/reader-npm-audit.json"
reader_audit_status=$?
set -e
popd >/dev/null

if (( govuln_status != 0 )); then
  echo "govulncheck found reachable vulnerabilities or failed: exit=$govuln_status" >&2
  exit 1
fi
if (( web_audit_status != 0 || reader_audit_status != 0 )); then
  echo "a production lock-file audit found vulnerabilities or failed" >&2
  exit 1
fi

for component in server web reader; do
  case "$component" in
    server) image="$SERVER_IMAGE" ;;
    web) image="$WEB_IMAGE" ;;
    reader) image="$READER_IMAGE" ;;
  esac
  tar_path="$output_dir/images/$component.tar"
  echo "exporting and scanning $component image: $image"
  docker image inspect "$image" >"$output_dir/reports/$component-image-inspect.json"
  docker save -o "$tar_path" "$image"

  docker run --rm \
    -v "$output_dir:/work" \
    -v "$output_dir/cache/trivy:/root/.cache/trivy" \
    "$TRIVY_IMAGE" image \
    --input "/work/images/$component.tar" \
    --severity HIGH,CRITICAL \
    --ignore-unfixed \
    --scanners vuln \
    --skip-version-check \
    --format json \
    --output "/work/reports/$component-trivy-image.json"

  docker run --rm \
    -v "$output_dir:/work" \
    "$SYFT_IMAGE" "docker-archive:/work/images/$component.tar" \
    -o "spdx-json=/work/sbom/$component.spdx.json" \
    -o "cyclonedx-json@1.6=/work/sbom/$component.cyclonedx.json"

  docker run --rm \
    -v "$output_dir:/work" \
    -v "$output_dir/cache/trivy:/root/.cache/trivy" \
    "$TRIVY_IMAGE" sbom "/work/sbom/$component.cyclonedx.json" \
    --severity HIGH,CRITICAL \
    --ignore-unfixed \
    --skip-version-check \
    --format json \
    --output "/work/reports/$component-trivy-sbom.json"

  image_vulnerabilities="$(jq '[.Results[]?.Vulnerabilities[]?] | length' \
    "$output_dir/reports/$component-trivy-image.json")"
  sbom_vulnerabilities="$(jq '[.Results[]?.Vulnerabilities[]?] | length' \
    "$output_dir/reports/$component-trivy-sbom.json")"
  if (( image_vulnerabilities != 0 || sbom_vulnerabilities != 0 )); then
    echo "$component contains fixable high/critical vulnerabilities" >&2
    exit 1
  fi

  jq '{
    component: $component,
    packages: (.packages | length),
    noAssertion: ([.packages[] | select(
      ((.licenseDeclared // "NOASSERTION") == "NOASSERTION") and
      ((.licenseConcluded // "NOASSERTION") == "NOASSERTION")
    )] | length),
    declaredLicenses: ([.packages[].licenseDeclared // "NOASSERTION"] | unique | sort),
    reviewCandidates: ([.packages[] |
      (.licenseDeclared // .licenseConcluded // "NOASSERTION") as $license |
      select($license | test("AGPL|GPL|LGPL|SSPL|BUSL|BSL|Commons Clause"; "i")) |
      {name, versionInfo, license: $license}
    ] | unique_by(.name, .versionInfo, .license))
  }' --arg component "$component" "$output_dir/sbom/$component.spdx.json" \
    >"$output_dir/reports/$component-license-summary.json"
done

{
  printf '{\n'
  printf '  "commit": "%s",\n' "$(git -C "$root_dir" rev-parse HEAD)"
  printf '  "generatedAt": "%s",\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf '  "govulncheck": "%s",\n' "$GOVULNCHECK_VERSION"
  printf '  "trivyImage": "%s",\n' "$TRIVY_IMAGE"
  printf '  "syftImage": "%s",\n' "$SYFT_IMAGE"
  printf '  "images": {\n'
  printf '    "server": "%s",\n' "$SERVER_IMAGE"
  printf '    "web": "%s",\n' "$WEB_IMAGE"
  printf '    "reader": "%s"\n' "$READER_IMAGE"
  printf '  }\n'
  printf '}\n'
} >"$output_dir/manifest.json"

(
  cd "$output_dir"
  find . -type f \
    ! -path './cache/*' \
    ! -path './images/*' \
    ! -name SHA256SUMS \
    -print0 | sort -z | xargs -0 shasum -a 256 >SHA256SUMS
)

echo "supply-chain verification passed: $output_dir"
