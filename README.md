# Caddy Proxy Manager Plus

**Make your websites easy to reach!** 🚀

This app helps you manage multiple websites and apps from one simple dashboard. Think of it like a **traffic director** for your internet services - it makes sure people get to the right place when they visit your websites.

**No coding required!** Just point, click, and you're done. ✨

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev/)
[![React Version](https://img.shields.io/badge/React-18.3-61DAFB?logo=react)](https://react.dev/)

---

## 🤔 What Does This Do?

**Simple Explanation:**
Imagine you have 5 different apps running on different computers in your house. Instead of remembering 5 complicated addresses, you can use one simple address like `myapps.com`, and this tool figures out where to send people based on what they're looking for.

**Real-World Example:**
- Someone types: `blog.mysite.com` → Goes to your blog server
- Someone types: `shop.mysite.com` → Goes to your online shop server
- All managed from one beautiful dashboard!

---

## ✨ What Can It Do?

- **🎨 Beautiful Dark Interface** - Easy on the eyes, works on phones and computers
- **🔄 Manage Multiple Websites** - Add, edit, or remove websites with a few clicks
- **🖥️ Connect Different Servers** - Works with servers anywhere (your closet, the cloud, anywhere!)
- **📥 Import Old Settings** - Already using Caddy? Bring your old setup right in
- **🔍 Test Before You Save** - Check if servers are reachable before going live
- **💾 Saves Everything Safely** - Your settings are stored securely
- **🔐 Secure Your Sites** - Add that green lock icon (HTTPS) to your websites
- **🌐 Works with Live Updates** - Perfect for chat apps and real-time features

---

## 📋 Quick Links

- 🏠 [**Start Here**](docs/getting-started.md) - Your first setup in 5 minutes
- 📚 [**All Documentation**](docs/index.md) - Find everything you need
- 📥 [**Import Guide**](docs/import-guide.md) - Bring in your existing setup
- 🐛 [**Report Problems**](https://github.com/Wikid82/CaddyProxyManagerPlus/issues) - We'll help!

---

## 🚀 The Super Easy Way to Start

**Want to skip all the technical stuff?** Use Docker! (It's like a magic app installer)

### Step 1: Get Docker
Don't have Docker? [Download it here](https://docs.docker.com/get-docker/) - it's free!

### Step 2: Run One Command
Open your terminal and paste this:

```bash
docker run -d \
  -p 8080:8080 \
  -v caddy_data:/app/data \
  --name caddy-proxy-manager \
  ghcr.io/wikid82/caddyproxymanagerplus:latest
```

### Step 3: Open Your Browser
Go to: **http://localhost:8080**

**That's it!** 🎉 You're ready to start adding your websites!

> 💡 **Tip:** Not sure what a terminal is? On Windows, search for "Command Prompt". On Mac, search for "Terminal".

---

## 🛠️ The Developer Way (If You Like Code)

Want to tinker with the app or help make it better? Here's how:

### What You Need First:
- **Go 1.22+** - [Get it here](https://go.dev/dl/) (the "engine" that runs the app)
- **Node.js 20+** - [Get it here](https://nodejs.org/) (helps build the pretty interface)

### Getting It Running:

1. **Download the app**
```bash
git clone https://github.com/Wikid82/CaddyProxyManagerPlus.git
cd CaddyProxyManagerPlus
```

2. **Start the "brain" (backend)**
```bash
cd backend
go mod download           # Gets the tools it needs
go run ./cmd/seed/main.go # Adds example data
go run ./cmd/api/main.go  # Starts the engine
```

3. **Start the "face" (frontend)** - Open a NEW terminal window
```bash
cd frontend
npm install     # Gets the tools it needs
npm run dev     # Shows you the interface
```

4. **See it work!**
   - Main app: http://localhost:3001
   - Backend: http://localhost:8080

### Quick Docker Way (Developers Too!)

```bash
docker-compose up -d
```

Opens at http://localhost:3001

---

## 🏗️ How It's Built (For Curious Minds)

**Don't worry if these words sound fancy - you don't need to know them to use the app!**

### The "Backend" (The Smart Part)
- **Go** - A fast programming language (like the app's brain)
- **Gin** - Helps handle web requests quickly
- **SQLite** - A tiny database (like a filing cabinet for your settings)

### The "Frontend" (The Pretty Part)
- **React** - Makes the buttons and forms look nice
- **TypeScript** - Keeps the code organized
- **TailwindCSS** - Makes everything pretty with dark mode

### Where Things Live

```
CaddyProxyManagerPlus/
├── backend/          ← The "brain" (handles your requests)
│   ├── cmd/          ← Starter programs
│   ├── internal/     ← The actual code
│   └── data/         ← Where your settings are saved
├── frontend/         ← The "face" (what you see and click)
│   ├── src/          ← The code for buttons and pages
│   └── coverage/     ← Test results (proves it works!)
└── docs/             ← Help guides (including this one!)
```

---

## ⚙️ Making Changes to the App (For Developers)

Want to add your own features or fix bugs? Here's how to work on the code:

### Working on the Backend (The Brain)

1. **Get the tools it needs**
```bash
cd backend
go mod download
```

2. **Set up the database** (adds example data to play with)
```bash
go run ./cmd/seed/main.go
```

3. **Make sure it works** (runs tests)
```bash
go test ./... -v
```

4. **Start it up**
```bash
go run ./cmd/api/main.go
```

Now the backend is running at `http://localhost:8080`

### Working on the Frontend (The Face)

1. **Get the tools it needs**
```bash
cd frontend
npm install
```

2. **Make sure it works** (runs tests)
```bash
npm test              # Keeps checking as you code
npm run test:ui       # Pretty visual test results
npm run test:coverage # Shows what's tested
```

3. **Start it up**
```bash
npm run dev
```

Now the frontend is running at `http://localhost:3001`

### Custom Settings (Optional)

Want to change ports or locations? Create these files:

**Backend Settings** (`backend/.env`):
```env
PORT=8080                    # Where the backend listens
DATABASE_PATH=./data/cpm.db  # Where to save data
LOG_LEVEL=debug              # How much detail to show
```

**Frontend Settings** (`frontend/.env`):
```env
VITE_API_URL=http://localhost:8080  # Where to find the backend
```

---

## 📡 Controlling the App with Code (For Developers)

Want to automate things or build your own tools? The app has an API (a way for programs to talk to it).

**What's an API?** Think of it like a robot that can do things for you. You send it commands, and it does the work!

### Things the API Can Do:

#### Check if it's alive
```http
GET /api/v1/health
```
Like saying "Hey, are you there?"

#### Manage Your Websites
```http
GET    /api/v1/proxy-hosts          # Show me all websites
POST   /api/v1/proxy-hosts          # Add a new website
GET    /api/v1/proxy-hosts/:uuid    # Show me one website
PUT    /api/v1/proxy-hosts/:uuid    # Change a website
DELETE /api/v1/proxy-hosts/:uuid    # Remove a website
```

#### Manage Your Servers
```http
GET    /api/v1/remote-servers          # Show me all servers
POST   /api/v1/remote-servers          # Add a new server
GET    /api/v1/remote-servers/:uuid    # Show me one server
PUT    /api/v1/remote-servers/:uuid    # Change a server
DELETE /api/v1/remote-servers/:uuid    # Remove a server
POST   /api/v1/remote-servers/:uuid/test # Is this server reachable?
```

#### Import Old Files
```http
GET    /api/v1/import/status          # How's the import going?
GET    /api/v1/import/preview         # Show me what will import
POST   /api/v1/import/upload          # Start importing a file
POST   /api/v1/import/commit          # Finish the import
DELETE /api/v1/import/cancel          # Cancel the import
```

**Want more details and examples?** Check out the [complete API guide](docs/api.md)!

---

## 🧪 Making Sure It Works (Testing)

**What's testing?** It's like double-checking your homework. We run automatic checks to make sure everything works before releasing updates!

### Checking the Backend

```bash
cd backend
go test ./... -v                    # Check everything
go test ./internal/api/handlers/... # Just check specific parts
go test -cover ./...                # Check and show what's covered
```

**Results**: ✅ 6 tests passing (all working!)

### Checking the Frontend

```bash
cd frontend
npm test                  # Keep checking as you work
npm run test:coverage     # Show me what's tested
npm run test:ui          # Pretty visual results
```

**Results**: ✅ 24 tests passing (~70% of code checked)
- Layout: 100% ✅ (fully tested)
- Import Table: 90% ✅ (almost fully tested)
- Forms: ~60% ✅ (mostly tested)

**What does this mean for you?** The app is reliable! We've tested it thoroughly so you don't have to worry.

---

## 🗄️ Where Your Settings Are Saved

**What's a database?** Think of it as a super organized filing cabinet where the app remembers all your settings!

The app saves:

- **Your Websites** - All the sites you've set up
- **Your Servers** - The computers you've connected
- **Your Caddy Files** - Original configuration files (if you imported any)
- **Security Stuff** - SSL certificates and who can access what
- **App Settings** - Your preferences and customizations
- **Import History** - What you've imported and when

**Want the technical details?** Check out the [database guide](docs/database-schema.md).

**Good news**: It's all saved in one tiny file, and you can back it up easily!

---

## 📥 Bringing In Your Old Caddy Files

Already using Caddy and have configuration files? No problem! You can import them:

**Super Simple Steps:**

1. **Click "Import"** in the app
2. **Upload your file** (or just paste the text)
3. **Look at what it found** - the app shows you what it understood
4. **Fix any conflicts** - if something already exists, choose what to do
5. **Click "Import"** - done!

**It's drag-and-drop easy!** The app figures out what everything means.

**Need help?** Read the [step-by-step import guide](docs/import-guide.md) with pictures and examples!

---

## 🔗 Helpful Links

- **📋 What We're Working On**: https://github.com/users/Wikid82/projects/7
- **🐛 Found a Problem?**: https://github.com/Wikid82/CaddyProxyManagerPlus/issues
- **💬 Questions?**: https://github.com/Wikid82/CaddyProxyManagerPlus/discussions

---

## 🤝 Want to Help Make This Better?

**We'd love your help!** Whether you can code or not, you can contribute:

**Ways You Can Help:**
- 🐛 Report bugs (things that don't work)
- 💡 Suggest new features (ideas for improvements)
- 📝 Improve documentation (make guides clearer)
- 🔧 Fix issues (if you know how to code)
- ⭐ Star the project (shows you like it!)

**If You Want to Add Code:**

1. **Make your own copy** (click "Fork" on GitHub)
2. **Make your changes** in a new branch
3. **Test your changes** to make sure nothing breaks
4. **Send us your changes** (create a "Pull Request")

**Don't worry if you're new!** We'll help you through the process. Check out our [Contributing Guide](CONTRIBUTING.md) for details.

---

## 📄 Legal Stuff (License)

This project is **free to use**! It's under the MIT License, which basically means:
- ✅ You can use it for free
- ✅ You can change it
- ✅ You can use it for your business
- ✅ You can share it

See the [LICENSE](LICENSE) file for the formal details.

---

## 🙏 Special Thanks

- Inspired by [Nginx Proxy Manager](https://nginxproxymanager.com/) (similar tool, different approach)
- Built with [Caddy Server](https://caddyserver.com/) (the power behind the scenes)
- Made beautiful with [TailwindCSS](https://tailwindcss.com/) (the styling magic)

---

## 💬 Questions?

**Stuck?** Don't be shy!
- 📖 Check the [documentation](docs/index.md)
- 💬 Ask in [Discussions](https://github.com/Wikid82/CaddyProxyManagerPlus/discussions)
- 🐛 Open an [Issue](https://github.com/Wikid82/CaddyProxyManagerPlus/issues) if something's broken

**We're here to help!** Everyone was a beginner once. 🌟

---

<p align="center">
  <strong>Version 0.1.0</strong><br>
  <em>Built with ❤️ by <a href="https://github.com/Wikid82">@Wikid82</a></em><br>
  <em>Made for humans, not just techies!</em>
</p>
