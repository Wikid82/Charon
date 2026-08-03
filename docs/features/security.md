# Security

Charon runs the front door to your home server — so keeping that door locked is not an optional feature, it's the whole point. This page explains, in plain language, how Charon keeps you and your apps safe. No jargon, no CVE numbers, just the big picture.

Looking for exact details on what changed in a specific update? Open the **"What's New"** popup (Settings > Appearance) and check its **Security** section — that's the place for release-by-release specifics. This page is the general "here's how we keep you safe, always" story.

## Our Philosophy: Safe by Default

You shouldn't need to be a security expert to run a secure server. Every protection described below is either turned on automatically or just a single click away — no config files, no command line, no guesswork.

## How Charon Protects You

### 🔐 Every Website Gets a Lock (Automatic HTTPS)

Charon automatically gets, installs, and renews free security certificates for every site you add — the same "padlock" you see on banking and shopping websites. You never have to think about expired certificates again.

### 🔑 Your Secrets Stay Secret (Encryption)

Passwords, API keys, and other sensitive settings you store in Charon — like credentials for your DNS provider or a notification webhook — are encrypted before they're saved. Even someone with direct access to Charon's storage can't read them in plain text. Backups can optionally be locked with your own passphrase too, so a copied backup file is useless without it.

### 🕵️ A Neighborhood Watch for Your Server (CrowdSec)

CrowdSec is like a neighborhood watch for the internet. It recognizes the tell-tale signs of an attacker — someone guessing passwords, scanning for weaknesses, and so on — and automatically blocks them. Because it shares threat information with a global community of other CrowdSec users, Charon often blocks an attacker before they've even tried anything against *your* server.

### 🚪 A Bouncer at Your Apps' Door (Forward Auth Gateway)

Some of the apps you run behind Charon don't have their own login screen. Turn on **Require Login** for one of those apps, and Charon stands in front of it like a bouncer — checking that a visitor is signed in before letting them through. Your app doesn't need to know anything changed; Charon handles the checking for it.

### 🧱 A Guard That Reads Every Request (Web Application Firewall)

The Web Application Firewall inspects incoming traffic for common attack patterns — the kind used to break into poorly-secured websites — and blocks them before they ever reach your app.

### 📋 A Guest List for Each App (Access Control Lists)

Want only certain people (or certain countries) to reach a site? Access Control Lists let you set an allow list or a block list per app, so you decide exactly who's let in.

### ⏱️ Slowing Down Troublemakers (Rate Limiting)

If someone starts hammering one of your sites with requests — trying to break in or just overloading it — rate limiting steps in and slows them down automatically, keeping the app available for everyone else.

### 🪪 Security Badges Modern Browsers Expect (Security Headers)

Charon automatically adds the security "badges" that modern web browsers look for on every site, the same ones large, professional websites use. One toggle gives your apps that same polished, trustworthy security posture.

### 📦 Verified, Tamper-Proof Deliveries (Supply Chain Security)

Every official release of Charon is digitally signed and comes with a complete parts list of everything inside it. That means you — or anyone — can verify that the copy of Charon you're running is exactly what the Charon team built, with nothing added or swapped along the way.

### 🔓 Locked Down Out of the Box (Container Hardening)

Charon doesn't run with full administrator rights inside its container — it uses only the narrow permissions it actually needs. That way, even in the unlikely event something goes wrong, the damage it could do is limited from the start.

## Reporting a Security Issue

Found something that looks like a security problem? We want to know. See [SECURITY.md](../SECURITY.md) for how to report it privately and safely.

## Learn More

- [CrowdSec Integration](crowdsec.md)
- [Web Application Firewall](waf.md)
- [Access Control Lists](access-control.md)
- [Rate Limiting](rate-limiting.md)
- [Security Headers](security-headers.md)
- [Supply Chain Security](supply-chain-security.md)

## Need Help?

- 💬 [Ask in Discussions](https://github.com/Wikid82/charon/discussions)
- 🐛 [Report Issues](https://github.com/Wikid82/Charon/issues)
- 📖 [View Full Documentation](https://wikid82.github.io/charon/)
