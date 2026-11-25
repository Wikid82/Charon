---
title: Encryption Key Rotation
description: How to change the key that protects your saved secrets, step by step
---

# Encryption Key Rotation

Charon saves some secrets for you, such as DNS provider logins and remote backup passwords. It locks them with an **encryption key**. Key rotation means swapping that lock for a new one, so the old key is no longer the only thing protecting your secrets.

You only need this when you want to change your key, for example on a schedule your own policy requires, or after you think an old key may have been seen by someone else.

## Your Keys

Charon reads its keys from settings (environment variables) in your Docker Compose file or your container setup.

| Setting name | What it is | Needed? |
| --- | --- | --- |
| `CHARON_ENCRYPTION_KEY` | The key that locks new secrets. | **Yes**, to save DNS logins, remote backup settings, tunnel settings, or uploaded certificate keys. |
| `CHARON_ENCRYPTION_KEY_NEXT` | The new key you are rotating to. | Only while you rotate. |
| `CHARON_ENCRYPTION_KEY_V1` to `CHARON_ENCRYPTION_KEY_V10` | Old keys. They keep older secrets readable. | Only if older secrets or older backups still need them. |

Only these exact names work. Other names, such as `ENCRYPTION_KEY`, are ignored. Charon does not read keys from a file.

### Making a New Key

Run this in a terminal and copy the result. It prints a random key:

```bash
openssl rand -base64 32
```

Save the key in a password manager right away. Never change a key by editing it, and never share it in chat or email.

## Before You Start

- You are logged in as an **admin**. Only admins can use the Encryption page.
- You have a **recent backup**. Go to **Tasks** → **Backups** and create one.
- You have saved the new key somewhere safe.

## Rotate Your Key

### Step 1: Add the New Key

Add the new key as `CHARON_ENCRYPTION_KEY_NEXT`. Keep your current key in `CHARON_ENCRYPTION_KEY`.

```yaml
environment:
  - CHARON_ENCRYPTION_KEY=your-current-key
  - CHARON_ENCRYPTION_KEY_NEXT=your-new-key
```

Restart Charon. From this point on, every secret you add or change is saved with the **new** key. Secrets saved before this step stay locked with the old key until you rotate in Step 3.

**Do not remove `CHARON_ENCRYPTION_KEY_NEXT` until the rotation is finished (Step 4).** Secrets saved with the new key can only be opened while that key is set. If you decide to stop the rotation, move the new key into a `V` slot (see Step 4) instead of deleting it. Otherwise those secrets become unreadable.

### Step 2: Check the Encryption Page

1. Open **Encryption** in the left menu.
2. Look at the **Keys** table. Both keys should be listed.
3. Look at the **Stored secrets** table. It shows how many saved secrets each part of Charon holds, and which key protects each one.

Optional: click **Validate Configuration**. It checks that every key you set can be read. If it reports a problem, check that you copied the key exactly.

### Step 3: Rotate

1. Click **Rotate Encryption Key**.
2. Read the message and click **Start Rotation**.

Charon re-locks every saved secret with the new key in one step. This covers:

- DNS providers
- DNS provider credentials
- Remote backup destinations
- Certificate private keys
- Tunnels
- Backup passphrase

When it finishes, a message shows how many secrets were updated. If the rotation fails, **nothing is changed**. You can try again.

Check the **Stored secrets** table. Your secrets should now be protected by the new key. If any row shows **unreadable**, see [Troubleshooting](#troubleshooting).

### Step 4: Make the New Key the Main Key

Charon needs the new key in the main slot, and the old key moved out of the way:

1. Set `CHARON_ENCRYPTION_KEY` to the **new** key.
2. Move the **old** key to `CHARON_ENCRYPTION_KEY_V1`. Any empty slot from V1 to V10 works.
3. Remove `CHARON_ENCRYPTION_KEY_NEXT`.
4. Restart Charon.

```yaml
environment:
  - CHARON_ENCRYPTION_KEY=your-new-key
  - CHARON_ENCRYPTION_KEY_V1=your-old-key
```

Your secrets keep working after this step, because Charon knows which key protects each one.

### Step 5: Check the Keys Table

Open **Encryption** again. The Keys table shows a status for each key:

- **Still needed**: at least one saved secret still uses this key. Keep it.
- **No longer needed**: no saved secret uses this key any more.
- **Could not be loaded**: Charon could not read this key. Check that you copied it correctly.

## How to Verify a Rotation Worked

Go through this list on the **Encryption** page:

- [ ] In the **Stored secrets** table, every row shows its secrets under the version number of your **new** key (see the Keys table).
- [ ] The **unreadable** count is **0** for every row.
- [ ] Click **Validate Configuration**. It reports no problems.
- [ ] After Step 4 and a restart, the **old** key shows **No longer needed** in the Keys table.

If any box does not match, do not remove any key. See [Troubleshooting](#troubleshooting).

## Removing an Old Key

Remove an old key only when its row says **No longer needed**.

1. Open **Encryption** and check the Keys table **right before** you remove the key. Do not rely on an earlier check.
2. Make an **offline copy** of the key first, such as in a password manager or on paper kept somewhere safe.
3. Remove the key's setting and restart Charon.

Keep the offline copy for as long as any backup made **before** the rotation may still be restored. Those backups need the old key to open their secrets.

## Things to Keep in Mind

### Downgrading Charon

If you go back to an older version of Charon after a rotation, the older version may not be able to read secrets protected by the new key. Keep **all** of your keys set until you are sure you will not go back.

### Old Backups

A backup made before a rotation needs the key that was current at that time. Keep the old keys in the `V` slots as long as you keep those backups.

### Undoing a Promotion

If something goes wrong after Step 4, put the old key back as `CHARON_ENCRYPTION_KEY` and the new key in a `V` slot. Restart Charon. Keep both keys set until everything works again.

## Troubleshooting

### Some secrets are marked "unreadable"

**What it means**: No key you have set can open those secrets. Charon does not delete them.

**What to do**:

1. If you removed a key recently, put it back and restart Charon.
2. If the key is lost, nothing can open the locked secrets again. Re-enter each one where you first set it up:
   - **DNS provider credentials**: on the DNS Providers page
   - **Remote backup destinations**: in the remote backup settings
   - **Tunnel credentials**: where you first set up the tunnel
   - **Uploaded certificates**: upload the certificate again, with its private key
   - **Backup passphrase**: enter it again in the Backup Encryption settings

### The Rotate button is greyed out

**What it means**: Every saved secret already uses the key that Charon would rotate to, so there is nothing to change.

**What to do**: To rotate to a new key, set `CHARON_ENCRYPTION_KEY_NEXT` to the new key and restart Charon.

The Rotate button can also turn on when no next key is set. This happens when some saved secrets are still locked with an older key, for example after you finished a rotation (Step 4) or restored a backup. Click **Rotate Encryption Key** to re-lock those secrets with your current key.

### DNS providers or the Encryption page are missing after a key change

**What it means**: Charon could not load your current key (`CHARON_ENCRYPTION_KEY`), so it turns off the features that need it. The Encryption page and DNS providers are not available until the key loads.

**What to do**:

1. Open the Charon log. Look for a message saying that encryption keys could not be loaded.
2. Check that `CHARON_ENCRYPTION_KEY` matches your saved key exactly. Watch for extra spaces or line breaks.
3. Fix the setting and restart Charon.

### Charon's log says some secrets need a key that is not set

**What it means**: You removed a key before the secrets were moved to the new key. Nothing was deleted.

**What to do**: Put the missing key back in its setting and restart Charon. Then try again.

### Rotation failed

**What to do**: Nothing was changed. Check that all your keys are set correctly, then try the rotation again. If it keeps failing, check the Charon log for details and ask for help on [GitHub Discussions](https://github.com/Wikid82/charon/discussions).

## Related

- [Disaster Recovery](disaster-recovery.md): what to do with your keys when you move Charon to a new machine
- [Backup & Restore](backup-restore.md): how backups work
