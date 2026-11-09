---
title: Sharing S3 Storage (Garage or MinIO) Through Charon
description: A beginner-friendly guide to making your home S3 storage (Garage or MinIO) reachable from the internet using a normal Charon Proxy Host.
---

# Sharing S3 Storage (Garage or MinIO) Through Charon

Do you run **Garage** or **MinIO** at home and want to reach it from outside your house? This guide shows you how to do that with Charon.

Good news: you do **not** need a special "S3" button. Charon treats your storage like any other website or app on your home network. You make a normal **Proxy Host**, and that's it.

---

## The Short Version

1. Make sure Charon can reach your storage machine (for example, over your VPN).
2. In Charon, click **Proxy Hosts**, then click **+ Add**.
3. Fill in the boxes as shown below and click **Save**.
4. Point your apps at your new public address.

---

## Words You Will See

| Word | What it means in plain English |
| --- | --- |
| **S3 storage** | A "filing cabinet" for files that apps can read and write over the internet. Garage and MinIO are two programs that provide one. |
| **Proxy Host** | A "doorman" that Charon sets up. People knock on the public address, and Charon walks them to the right machine inside your home. |
| **Domain name** | The public address people type, such as `s3.example.com`. |
| **IP address** | The number that identifies a machine on a network, such as `10.8.0.5`. |
| **Port** | A numbered "door" on a machine. Garage uses door `3900` for S3 and MinIO uses door `9000`. |
| **VPN** | A private tunnel that links your machines together, even when they are in different places. |

---

## Before You Start

You need:

- Charon installed and running, and you can open its web page.
- A domain name (such as `example.com`) that you control, and the ability to add a new record for it at your domain provider. If you don't have one yet, see the [Getting Started](../getting-started.md) page.
- Garage or MinIO running on your storage machine.
- Your storage machine reachable from the Charon machine. A VPN link works well for this.

> **Tip:** Find the storage machine's address on your VPN. It often looks like `10.x.x.x` or `100.x.x.x`. Use that address in Charon, not your home's public internet address.

---

## Step 1: Create the Proxy Host

1. Open Charon in your web browser.
2. Click **Proxy Hosts**, then click **+ Add**.
3. Fill in the boxes like this:

| Box | What to type | Example |
| --- | --- | --- |
| **Domain** | The public address you want to use | `s3.example.com` |
| **Forward To** | The address of your storage machine, reachable from Charon | `10.8.0.5` |
| **Port** | `3900` for Garage, `9000` for MinIO | `3900` |
| **Scheme** | `http` | `http` |

4. Keep the secure (HTTPS) option turned on. Charon gets your certificate automatically.
5. Click **Save**.

Charon will now answer on `https://s3.example.com` and pass every request to your storage machine.

> **About the "private IP address" warning:** Charon may show a warning because `10.8.0.5` is a private address. This warning is only a reminder. If the address is a VPN address that Charon can reach, you can safely ignore it.

---

## Step 2: Point Your Domain at Charon

Go to your domain provider and create an **A record** (a record that says "this name lives at this address"):

| Name | Points to |
| --- | --- |
| `s3` | The public internet address of your Charon machine |

After a few minutes, `s3.example.com` will lead to Charon.

---

## Step 3: Connect Your Apps

Use these settings in your app (or in a tool such as rclone or the AWS command-line tool):

| Setting | Value |
| --- | --- |
| **Endpoint URL** | `https://s3.example.com` |
| **Region** | The region name you chose when setting up Garage or MinIO |
| **Access key and secret key** | The keys you created inside Garage or MinIO |
| **Addressing style** | **Path-style** (this is the simplest choice and needs no extra steps) |

That's it. Your apps can now upload, download, and list files through your new address.

---

## Optional: Use Bucket-Style Addresses

Some apps prefer addresses like `my-photos.s3.example.com` instead of `s3.example.com/my-photos`. If you want that, you need a few extra steps. If the simple path-style setup above works for you, skip this section.

1. **Add a wildcard name to your Proxy Host.** In the **Domain** box, type both names separated by a comma:

   ```text
   s3.example.com,*.s3.example.com
   ```

   The `*` means "anything before this part," so every bucket gets its own name.

2. **Get a wildcard certificate.** A wildcard certificate covers every name that starts with `*.`. Charon gets one by proving you own your domain through your DNS provider. Follow the [DNS Challenge](../features/dns-challenge.md) page and the [DNS provider setup](dns-providers.md) guide for your provider. The wildcard name is only covered by a DNS Challenge certificate, so you must turn that on for this host.

3. **Tell Garage about your domain.** In Garage's settings file, set the root domain to match your public name:

   ```text
   root_domain = ".s3.example.com"
   ```

   Restart Garage after you change this.

4. **Add a wildcard record at your domain provider.** Create an **A record** with the name `*.s3` that points to the same address as `s3`.

---

## Keeping Your Storage Safe

Your storage is now reachable from the internet, so take these steps seriously:

- **Keep the VPN link private.** Only your own machines should be on that tunnel. Do not open the storage machine's own ports to the internet.
- **Only share the public address through Charon.** Let Charon be the one door to the outside world.
- **Use strong keys.** Create long, random access keys inside Garage or MinIO. Don't reuse a password you use elsewhere.
- **Consider Charon's protection features for this host.** Charon has [Access Control Lists](../features/access-control.md) that limit who can reach a site, and a [Security overview](../features/security.md) that explains the other protections it provides. Turn on what fits your needs.

---

## Troubleshooting

### My upload or download works in one tool but fails in another

Some tools are picky about how they talk to Garage. If you see an error that mentions **"signed header content-length is not present"** while using the AWS command-line tool to download files from Garage, this is a known issue between that tool and Garage. Charon is not the cause, and Charon cannot change it. Try one of these fixes:

- **Use rclone.** It works well with both Garage and MinIO through Charon.
- **Use the classic transfer mode in the AWS tool.** Run this once in a terminal:

  ```bash
  aws configure set default.s3.preferred_transfer_client classic
  ```

- **Use the single-file download command.** Use `aws s3api get-object` instead of `aws s3 cp`.

MinIO does not have this problem.

### The AWS command-line tool says "invalid checksum algorithm"

Newer versions of the AWS tool add extra checksum data that older Garage versions (such as Garage 1.0.1) reject. This happens even without Charon. To fix it, set these two settings before running the tool:

```bash
export AWS_REQUEST_CHECKSUM_CALCULATION=when_required
export AWS_RESPONSE_CHECKSUM_VALIDATION=when_required
```

On Windows PowerShell, use `$env:AWS_REQUEST_CHECKSUM_CALCULATION="when_required"` and `$env:AWS_RESPONSE_CHECKSUM_VALIDATION="when_required"` instead.

### The page does not load at all

- Check that **Forward To** and **Port** match where Garage or MinIO is actually running.
- Check that your VPN is connected on the Charon machine.
- Check that your domain's A record points to the right public address.
