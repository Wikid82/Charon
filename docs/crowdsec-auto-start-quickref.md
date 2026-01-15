# CrowdSec Auto-Start - Quick Reference

**Version:** v0.9.0+
**Last Updated:** December 23, 2025

---

## 🚀 What's New

CrowdSec now **automatically starts** when the container restarts (if it was previously enabled).

---

## ✅ Verification (One Command)

```bash
docker exec charon cscli lapi status
```

**Expected:** `✓ You can successfully interact with Local API (LAPI)`

---

## 🔧 Enable CrowdSec

1. Open Security dashboard
2. Toggle CrowdSec **ON**
3. Wait 10-15 seconds

**Done!** CrowdSec will auto-start on future restarts.

---

## 🔄 After Container Restart

```bash
docker restart charon
sleep 15
docker exec charon cscli lapi status
```

**If working:** CrowdSec shows "Active"
**If not working:** See troubleshooting below

---

## ⚠️ Troubleshooting (3 Steps)

### 1. Check Logs

```bash
docker logs charon 2>&1 | grep "CrowdSec reconciliation"
```

### 2. Check Mode

```bash
docker exec charon sqlite3 /app/data/charon.db \
  "SELECT crowdsec_mode FROM security_configs LIMIT 1;"
```

**Expected:** `local`

### 3. Manual Start

```bash
curl -X POST http://localhost:8080/api/v1/admin/crowdsec/start
```

---

## 📖 Full Documentation

- **Implementation Details:** [crowdsec_startup_fix_COMPLETE.md](implementation/crowdsec_startup_fix_COMPLETE.md)
- **Migration Guide:** [migration-guide-crowdsec-auto-start.md](migration-guide-crowdsec-auto-start.md)
- **User Guide:** [getting-started.md](getting-started.md#step-15-database-migrations-if-upgrading)

---

## 🆘 Get Help

**GitHub Issues:** [Report Problems](https://github.com/Wikid82/charon/issues)

---

*Quick reference for v0.9.0+ CrowdSec auto-start behavior*
