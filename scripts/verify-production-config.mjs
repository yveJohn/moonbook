let input = "";
for await (const chunk of process.stdin) input += chunk;
const config = JSON.parse(input);
const required = ["postgres", "redis", "minio", "minio-init", "migrate", "server", "web", "reader-ui", "gateway"];
const errors = [];

for (const name of required) {
  const service = config.services?.[name];
  if (!service) {
    errors.push(`${name}: missing service`);
    continue;
  }
  if (!/@sha256:[a-f0-9]{64}$/i.test(service.image ?? "")) {
    errors.push(`${name}: image is not pinned by sha256 digest`);
  }
  if (service.build) {
    errors.push(`${name}: production config still contains build`);
  }
  if (name !== "minio-init") {
    if (!service.deploy?.resources?.limits?.cpus || !service.deploy?.resources?.limits?.memory) {
      errors.push(`${name}: CPU or memory limit is missing`);
    }
    if (!service.deploy?.resources?.limits?.pids) {
      errors.push(`${name}: PID limit is missing`);
    }
  }
  if (service.logging?.driver !== "json-file" || !service.logging?.options?.["max-size"] || !service.logging?.options?.["max-file"]) {
    errors.push(`${name}: json-file log rotation is incomplete`);
  }
}

for (const [name, service] of Object.entries(config.services ?? {})) {
  for (const port of service.ports ?? []) {
    if (port.host_ip && port.host_ip !== "127.0.0.1") {
      errors.push(`${name}: published port is not bound to 127.0.0.1`);
    }
  }
}

const proxies = config.services?.server?.environment?.MOONBOOK_TRUSTED_PROXIES ?? "";
if (!proxies.trim()) {
  errors.push("server: MOONBOOK_TRUSTED_PROXIES is empty");
}
if (proxies.split(",").some((proxy) => proxy.trim() === "0.0.0.0/0" || proxy.trim() === "::/0")) {
  errors.push("server: MOONBOOK_TRUSTED_PROXIES trusts every address");
}

if (errors.length) {
  for (const error of errors) console.error(error);
  process.exit(1);
}

console.log(`production compose contract passed for ${required.length} services`);
