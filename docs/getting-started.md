# 🏠 Getting Started with Caddy Proxy Manager Plus

**Welcome!** This guide will walk you through setting up your first proxy. Don't worry if you're new to this - we'll explain everything step by step!

---

## 🤔 What Is This App?

Think of this app as a **traffic controller** for your websites and apps.

**Here's a simple analogy:**
Imagine you have several houses (websites/apps) on different streets (servers). Instead of giving people complicated directions to each house, you have one main address (your domain) where a helpful guide (the proxy) sends visitors to the right house automatically.

**What you can do:**
- ✅ Make multiple websites accessible through one domain
- ✅ Route traffic from example.com to different servers
- ✅ Manage SSL certificates (the lock icon in browsers)
- ✅ Control who can access what

---

## 📋 Before You Start

You'll need:
1. **A computer** (Windows, Mac, or Linux)
2. **Docker installed** (it's like a magic box that runs apps)
   - Don't have it? [Get Docker here](https://docs.docker.com/get-docker/)
3. **5 minutes** of your time

That's it! No programming needed.

---

### Step 1: Get the App Running

### The Easy Way (Recommended)

Open your **terminal** (or Command Prompt on Windows) and paste this:

```bash
docker run -d \
  -p 8080:8080 \
  -v caddy_data:/app/data \
  --name caddy-proxy-manager \
  ghcr.io/wikid82/cpmp:latest
```

**What does this do?** It downloads and starts the app. You don't need to understand the details - just copy and paste!

### Check If It's Working

1. Open your web browser
2. Go to: `http://localhost:8080`
3. You should see the app! 🎉

> **Didn't work?** Check if Docker is running. On Windows/Mac, look for the Docker icon in your taskbar.

---

## 🎯 Step 2: Create Your First Proxy Host

Let's set up your first proxy! We'll create a simple example.

### What's a Proxy Host?

A **Proxy Host** is like a forwarding address. When someone visits `mysite.com`, it secretly sends them to `192.168.1.100:3000` without them knowing.

### Let's Create One!

1. **Click "Proxy Hosts"** in the left sidebar
2. **Click "+ Add Proxy Host"** button (top right)
3. **Fill in the form:**

   📝 **Domain Name:** (What people type in their browser)
   ```
   myapp.local
   ```
   > This is like your house's street address

   📍 **Forward To:** (Where the traffic goes)
   ```
   192.168.1.100
   ```
   > This is where your actual app is running

   🔢 **Port:** (Which door to use)
   ```
   3000
   ```
   > Apps listen on specific "doors" (ports) - 3000 is common for web apps

   🌐 **Scheme:** (How to talk to it)
   ```
   http
   ```
   > Choose `http` for most apps, `https` if your app already has SSL

4. **Click "Save"**

**Congratulations!** 🎉 You just created your first proxy! Now when you visit `http://myapp.local`, it will show your app from `192.168.1.100:3000`.

---

## 🌍 Step 3: Set Up a Remote Server (Optional)

Sometimes your apps are on different computers (servers). Let's add one!

### What's a Remote Server?

Think of it as **telling the app about other computers** you have. Once added, you can easily send traffic to them.

### Adding a Remote Server

1. **Click "Remote Servers"** in the left sidebar
2. **Click "+ Add Server"** button
3. **Fill in the details:**

   🏷️ **Name:** (A friendly name)
   ```
   My Home Server
   ```

   🌐 **Hostname:** (The address of your server)
   ```
   192.168.1.50
   ```

   📝 **Description:** (Optional - helps you remember)
   ```
   The server in my office running Docker
   ```

4. **Click "Test Connection"** - this checks if the app can reach your server
5. **Click "Save"**

Now when creating proxy hosts, you can pick this server from a dropdown instead of typing the address every time!

---

## 📥 Step 4: Import Existing Caddy Files (If You Have Them)

Already using Caddy and have configuration files? You can bring them in!

### What's a Caddyfile?

It's a **text file that tells Caddy how to route traffic**. If you're not sure if you have one, you probably don't need this step.

### How to Import

1. **Click "Import Caddy Config"** in the left sidebar
2. **Choose your method:**
   - **Drag & Drop:** Just drag your `Caddyfile` into the box
   - **Paste:** Copy the contents and paste them in the text area
3. **Click "Parse Config"** - the app reads your file
4. **Review the results:**
   - ✅ Green items = imported successfully
   - ⚠️ Yellow items = need your attention (conflicts)
   - ❌ Red items = couldn't import (will show why)
5. **Resolve any conflicts** (the app will guide you)
6. **Click "Import Selected"**

Done! Your existing setup is now in the app.

> **Need more help?** Check the detailed [Import Guide](import-guide.md)

---

## 💡 Tips for New Users

### 1. Start Small
Don't try to import everything at once. Start with one proxy host, make sure it works, then add more.

### 2. Use Test Connection
When adding remote servers, always click "Test Connection" to make sure the app can reach them.

### 3. Check Your Ports
Make sure the ports you use aren't already taken by other apps. Common ports:
- `80` - Web traffic (HTTP)
- `443` - Secure web traffic (HTTPS)
- `3000-3999` - Apps often use these
- `8080-8090` - Alternative web ports

### 4. Local Testing First
Test everything with local addresses (like `localhost` or `192.168.x.x`) before using real domain names.

### 5. Save Backups
The app stores everything in a database. The Docker command above saves it in `caddy_data` - don't delete this!

---

## 🐛 Something Not Working?

### App Won't Start
- **Check if Docker is running** - look for the Docker icon
- **Check if port 8080 is free** - another app might be using it
- **Try:** `docker ps` to see if it's running

### Can't Access the Website
- **Check your spelling** - domain names are picky
- **Check the port** - make sure the app is actually running on that port
- **Check the firewall** - might be blocking connections

### Import Failed
- **Check your Caddyfile syntax** - paste it at [Caddy Validate](https://caddyserver.com/docs/caddyfile)
- **Look at the error message** - it usually tells you what's wrong
- **Start with a simple file** - test with just one site first

---

## 📚 What's Next?

You now know the basics! Here's what to explore:

- 🔐 **Add SSL Certificates** - get the green lock icon
- 🚦 **Set Up Access Lists** - control who can visit your sites
- ⚙️ **Configure Settings** - customize the app
- 🔌 **Try the API** - control everything with code

---

## 🆘 Still Need Help?

We're here for you!

- 💬 [Ask on GitHub Discussions](https://github.com/Wikid82/CaddyProxyManagerPlus/discussions)
- 🐛 [Report a Bug](https://github.com/Wikid82/CaddyProxyManagerPlus/issues)
- 📖 [Read the Full Documentation](index.md)

---

<p align="center">
  <strong>You're doing great! 🌟</strong><br>
  <em>Remember: Everyone was a beginner once. Take your time and have fun!</em>
</p>
