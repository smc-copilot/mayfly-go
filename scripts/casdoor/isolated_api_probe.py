#!/usr/bin/env python3
"""Probe a Casdoor image with disposable, isolated Docker resources.

Input: a base64-encoded JSON init-data template on stdin. Set CASDOOR_IMAGE and
MYSQL_IMAGE to locally available test images. All credentials are generated
for this run. The script never mounts production configuration or a production DB.
"""
import base64
import json
import os
import re
import secrets
import shutil
import subprocess
import sys
import tempfile
import time
import urllib.parse

CASDOOR_IMAGE = os.environ.get("CASDOOR_IMAGE", "")
MYSQL_IMAGE = os.environ.get("MYSQL_IMAGE", "")


def docker(*args, check=True):
    result = subprocess.run(["docker", *args], stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=60)
    if check and result.returncode:
        raise RuntimeError("docker " + args[0] + " failed")
    return result


def main():
    if not CASDOOR_IMAGE or not MYSQL_IMAGE:
        raise ValueError("CASDOOR_IMAGE and MYSQL_IMAGE must specify local test images")
    failed = False
    seed = json.loads(base64.b64decode(input().strip()))
    organization = seed["organizations"][0]
    template = seed["applications"][0]
    for key, value in list(seed.items()):
        if isinstance(value, list):
            seed[key] = []

    org_name = "mayflytest"
    organization.update(owner="admin", name=org_name, displayName="Mayfly isolated test")
    client_id = secrets.token_hex(12)
    client_secret = secrets.token_hex(24)
    db_password = secrets.token_hex(20)
    apps = []
    for index in range(3):
        item = dict(template)
        item.update(owner="admin", name="fixture-app-" + str(index), organization=org_name)
        item["clientId"] = client_id if index == 0 else secrets.token_hex(12)
        item["clientSecret"] = client_secret if index == 0 else secrets.token_hex(24)
        apps.append(item)
    seed["organizations"] = [organization]
    seed["applications"] = apps

    marker = secrets.token_hex(8)
    prefix = "mayfly-casdoor-isolation-" + marker
    network, db, app = prefix + "-net", prefix + "-db", prefix + "-app"
    label = "io.mayfly.isolated=" + marker
    temp_dir = tempfile.mkdtemp(prefix=prefix + "-", dir="/tmp")
    candidates = [app, db]

    def redact(value):
        for secret in (db_password, client_id, client_secret, *(item["clientId"] for item in apps), *(item["clientSecret"] for item in apps)):
            value = value.replace(secret, "[redacted]")
        return value

    def cleanup():
        for name in candidates:
            result = docker("inspect", name, "--format",
                            '{{ index .Config.Labels "io.mayfly.isolated" }}', check=False)
            if result.returncode == 0 and result.stdout.decode().strip() == marker:
                removed = docker("rm", "-f", name, check=False)
                print("cleanup container", name.rsplit("-", 1)[-1], removed.returncode == 0, flush=True)
        result = docker("network", "inspect", network, "--format",
                        '{{ index .Labels "io.mayfly.isolated" }}', check=False)
        if result.returncode == 0 and result.stdout.decode().strip() == marker:
            print("cleanup network", docker("network", "rm", network, check=False).returncode == 0, flush=True)
        resolved = os.path.realpath(temp_dir)
        if os.path.dirname(resolved) == "/tmp" and os.path.basename(resolved).startswith(prefix + "-"):
            shutil.rmtree(resolved)
            print("cleanup files true", flush=True)

    try:
        config = docker("run", "--rm", "--network", "none", "--entrypoint", "cat",
                        CASDOOR_IMAGE, "/conf/app.conf").stdout.decode()
        replacements = {
            "drivername": "mysql",
            "datasourcename": "root:" + db_password + "@tcp(" + db + ":3306)/",
            "dbname": "casdoor",
            "initdatafile": "/conf/init_data.json",
            "httpport": "8000",
        }
        seen, lines = set(), []
        for line in config.splitlines():
            match = re.match(r"^\s*([A-Za-z][A-Za-z0-9_]*)\s*=", line)
            if match and match.group(1).lower() in replacements:
                key = match.group(1).lower()
                lines.append(match.group(1) + " = " + replacements[key])
                seen.add(key)
            else:
                lines.append(line)
        for key, value in replacements.items():
            if key not in seen:
                lines.append(key + " = " + value)
        with open(os.path.join(temp_dir, "app.conf"), "w", encoding="utf-8") as file:
            file.write("\n".join(lines) + "\n")
        with open(os.path.join(temp_dir, "init_data.json"), "w", encoding="utf-8") as file:
            json.dump(seed, file)
        os.chmod(os.path.join(temp_dir, "app.conf"), 0o600)
        os.chmod(os.path.join(temp_dir, "init_data.json"), 0o600)

        docker("network", "create", "--internal", "--label", label, network)
        docker("run", "-d", "--name", db, "--label", label, "--network", network,
               "--memory", "768m", "--cpus", "1", "-e", "MYSQL_ROOT_PASSWORD=" + db_password,
               "-e", "MYSQL_DATABASE=casdoor", MYSQL_IMAGE)
        print("isolated database initializing", flush=True)
        for _ in range(90):
            probe = docker("exec", db, "mysqladmin", "ping", "--protocol=tcp", "--host=127.0.0.1",
                           "-uroot", "--password=" + db_password, "--silent", check=False)
            if probe.returncode == 0:
                break
            time.sleep(1)
        else:
            raise RuntimeError("isolated MySQL TCP endpoint not ready")

        docker("run", "-d", "--name", app, "--label", label, "--network", network,
               "--memory", "768m", "--cpus", "1", "--user", "0:0",
               "-v", temp_dir + ":/conf:ro", CASDOOR_IMAGE)
        print("isolated casdoor initializing", flush=True)
        ready = False
        state = ""
        for _ in range(60):
            probe = docker("exec", app, "curl", "-fsS", "--max-time", "2",
                           "http://127.0.0.1:8000/api/health", check=False)
            if probe.returncode == 0:
                try:
                    if json.loads(probe.stdout).get("status") == "ok":
                        ready = True
                        break
                except json.JSONDecodeError:
                    pass
            state = docker("inspect", app, "--format", "{{.State.Status}}", check=False).stdout.decode().strip()
            if state == "exited":
                break
            time.sleep(1)
        if not ready:
            log = docker("logs", "--tail", "20", app, check=False)
            print("isolated startup log:", redact((log.stdout + log.stderr).decode(errors="replace"))[-3000:], flush=True)
            raise RuntimeError("isolated Casdoor not ready")

        token = base64.b64encode((client_id + ":" + client_secret).encode()).decode()

        def api(method, path, payload=None):
            args = ["exec", app, "curl", "-fsS", "--max-time", "10", "--request", method,
                    "--header", "Authorization: Basic " + token]
            if payload is not None:
                args.extend(["--header", "Content-Type: application/json", "--data", json.dumps(payload)])
            args.append("http://127.0.0.1:8000" + path)
            response = docker(*args, check=False)
            if response.returncode:
                return {"status": "transport_error"}
            try:
                return json.loads(response.stdout)
            except json.JSONDecodeError:
                return {"status": "invalid_response"}

        q = urllib.parse.urlencode({"organization": org_name, "owner": "admin", "p": 1, "pageSize": 10})
        result = api("GET", "/api/get-organization-applications?" + q)
        app_data = result.get("data")
        checks = [result.get("status") == "ok" and isinstance(app_data, list) and len(app_data) == 3]
        print("app_read:", result.get("status"), "rows:", len(app_data) if isinstance(app_data, list) else "unknown", flush=True)
        app_id = "admin/fixture-app-1"
        before = api("GET", "/api/get-application?" + urllib.parse.urlencode({"id": app_id}))
        app_before = before.get("data") or {}
        q = urllib.parse.urlencode({"id": app_id, "columns": "displayName"})
        result = api("POST", "/api/update-application?" + q,
                     {"owner": "admin", "name": "fixture-app-1", "organization": org_name,
                      "clientId": apps[1]["clientId"], "displayName": "Updated app fixture"})
        check = api("GET", "/api/get-application?" + urllib.parse.urlencode({"id": app_id}))
        app_after = check.get("data") or {}
        checks.append(before.get("status") == "ok" and result.get("status") == "ok" and check.get("status") == "ok"
                      and app_after.get("displayName") == "Updated app fixture"
                      and app_before.get("clientId") == apps[1]["clientId"]
                      and app_after.get("clientId") == app_before.get("clientId")
                      and app_before.get("clientSecret") == apps[1]["clientSecret"]
                      and app_after.get("clientSecret") == app_before.get("clientSecret"))
        print("app_partial_update:", result.get("status"), "readback_matches:",
              app_after.get("displayName") == "Updated app fixture", "secret_verified:",
              app_before.get("clientSecret") == apps[1]["clientSecret"] and app_after.get("clientSecret") == app_before.get("clientSecret"), "reason:",
              redact(str(result.get("msg", "")))[:200], flush=True)
        q = urllib.parse.urlencode({"owner": org_name, "p": 1, "pageSize": 10})
        result = api("GET", "/api/get-users?" + q)
        checks.append(result.get("status") == "ok")
        print("user_read:", result.get("status"), flush=True)
        username = "fixture-" + secrets.token_hex(4)
        user_id = org_name + "/" + username
        result = api("POST", "/api/add-user", {"owner": org_name, "name": username, "displayName": "Fixture"})
        checks.append(result.get("status") == "ok")
        print("user_create:", result.get("status"), flush=True)
        q = urllib.parse.urlencode({"id": user_id})
        result = api("GET", "/api/get-user?" + q)
        print("user_readback:", result.get("status"), "exists:", bool(result.get("data")), flush=True)
        checks.append(result.get("status") == "ok" and bool(result.get("data")))
        q = urllib.parse.urlencode({"id": user_id, "columns": "displayName"})
        result = api("POST", "/api/update-user?" + q, {"displayName": "Updated fixture"})
        check = api("GET", "/api/get-user?" + urllib.parse.urlencode({"id": user_id}))
        checks.append(result.get("status") == "ok" and check.get("status") == "ok"
                      and (check.get("data") or {}).get("displayName") == "Updated fixture")
        print("user_partial_update:", result.get("status"), "readback_matches:",
              (check.get("data") or {}).get("displayName") == "Updated fixture", flush=True)
        role_name = "fixture-role-" + secrets.token_hex(4)
        role_id = org_name + "/" + role_name
        result = api("POST", "/api/add-role", {"owner": org_name, "name": role_name,
                     "displayName": "Fixture role", "users": [user_id], "roles": []})
        check = api("GET", "/api/get-role?" + urllib.parse.urlencode({"id": role_id}))
        checks.append(result.get("status") == "ok" and check.get("status") == "ok"
                      and user_id in ((check.get("data") or {}).get("users") or []))
        print("role_create:", result.get("status"), "readback_member:",
              user_id in ((check.get("data") or {}).get("users") or []), flush=True)
        q = urllib.parse.urlencode({"id": role_id, "columns": "users"})
        result = api("POST", "/api/update-role?" + q, {"users": []})
        check = api("GET", "/api/get-role?" + urllib.parse.urlencode({"id": role_id}))
        checks.append(result.get("status") == "ok" and check.get("status") == "ok"
                      and (check.get("data") or {}).get("users") == [])
        print("role_partial_update:", result.get("status"), "readback_empty:",
              (check.get("data") or {}).get("users") == [], flush=True)
        if not all(checks):
            failed = True
            print("isolated checks failed:", len(checks) - sum(checks), "of", len(checks), flush=True)
        else:
            print("isolated checks passed:", len(checks), flush=True)
    except Exception as exc:
        failed = True
        print("isolated test failed:", type(exc).__name__, redact(str(exc)), flush=True)
    finally:
        cleanup()
    if failed:
        sys.exit(1)


if __name__ == "__main__":
    main()
