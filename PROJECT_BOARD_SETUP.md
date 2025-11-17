# GitHub Project Board Setup & Automation Guide

This guide will help you set up the project board and automation for CaddyProxyManager+.

## 🎯 Quick Start (5 Minutes)

### Step 1: Create the Project Board

1. Go to https://github.com/Wikid82/CaddyProxyManagerPlus/projects
2. Click **"New project"**
3. Choose **"Board"** view
4. Name it: `CaddyProxyManager+ Development`
5. Click **"Create"**

### Step 2: Configure Project Columns

The new GitHub Projects automatically creates columns. Add these views/columns:

#### Recommended Column Setup:
1. **📋 Backlog** - Issues that are planned but not started
2. **🏗️ Alpha** - Core foundation features (v0.1)
3. **🔐 Beta - Security** - Authentication, WAF, CrowdSec features
4. **📊 Beta - Monitoring** - Logging, dashboards, analytics
5. **🎨 Beta - UX** - UI improvements and user experience
6. **🚧 In Progress** - Currently being worked on
7. **👀 Review** - Ready for code review
8. **✅ Done** - Completed issues

### Step 3: Get Your Project Number

After creating the project, your URL will look like:
```
https://github.com/users/Wikid82/projects/1
```

The number at the end (e.g., `1`) is your **PROJECT_NUMBER**.

### Step 4: Update the Automation Workflow

1. Open `.github/workflows/auto-add-to-project.yml`
2. Replace `YOUR_PROJECT_NUMBER` with your actual project number:
   ```yaml
   project-url: https://github.com/users/Wikid82/projects/1
   ```
3. Commit and push the change

### Step 5: Create Labels

1. Go to: https://github.com/Wikid82/CaddyProxyManagerPlus/actions
2. Find the workflow: **"Create Project Labels"**
3. Click **"Run workflow"** > **"Run workflow"**
4. Wait ~30 seconds - this creates all 27 labels automatically!

### Step 6: Test the Automation

1. Create a test issue with title: `[ALPHA] Test Issue`
2. Watch it automatically:
   - Get labeled with `alpha`
   - Get added to your project board
   - Appear in the correct column

---

## 🔧 How the Automation Works

### Workflow 1: `auto-add-to-project.yml`
**Triggers**: When an issue or PR is opened/reopened
**Action**: Automatically adds it to your project board

### Workflow 2: `auto-label-issues.yml`
**Triggers**: When an issue is opened or edited
**Action**: Scans title and body for keywords and adds labels

**Auto-labeling Examples:**
- Title contains `[critical]` → Adds `critical` label
- Body contains `crowdsec` → Adds `crowdsec` label
- Title contains `[alpha]` → Adds `alpha` label

### Workflow 3: `create-labels.yml`
**Triggers**: Manual only
**Action**: Creates all project labels with proper colors and descriptions

---

## 📝 Using the Issue Templates

We've created 4 specialized issue templates:

### 1. 🏗️ Alpha Feature (`alpha-feature.yml`)
For core foundation features (Issues #1-10 in planning doc)
- Automatically tagged with `alpha` and `feature`
- Includes priority selector
- Has task checklist format

### 2. 🔐 Beta Security Feature (`beta-security-feature.yml`)
For authentication, WAF, CrowdSec, etc. (Issues #11-22)
- Automatically tagged with `beta`, `feature`, `security`
- Includes threat model section
- Security testing plan included

### 3. 📊 Beta Monitoring Feature (`beta-monitoring-feature.yml`)
For logging, dashboards, analytics (Issues #23-27)
- Automatically tagged with `beta`, `feature`, `monitoring`
- Includes metrics planning
- UI/UX considerations section

### 4. ⚙️ General Feature (`general-feature.yml`)
For any other feature request
- Flexible milestone selection
- Problem/solution format
- User story section

---

## 🎨 Label System

### Priority Labels (Required for all issues)
- 🔴 **critical** - Must have, blocks other work
- 🟠 **high** - Important, should be included
- 🟡 **medium** - Nice to have, can be deferred
- 🟢 **low** - Future enhancement

### Milestone Labels
- 🟣 **alpha** - Foundation (v0.1)
- 🔵 **beta** - Advanced features (v0.5)
- 🟦 **post-beta** - Future enhancements (v1.0+)

### Category Labels
- **architecture**, **backend**, **frontend**
- **security**, **ssl**, **sso**, **waf**
- **caddy**, **crowdsec**
- **database**, **ui**, **deployment**
- **monitoring**, **documentation**, **testing**
- **performance**, **community**
- **plus** (premium features), **enterprise**

---

## 📊 Creating Issues from Planning Doc

### Method 1: Manual Creation (Recommended for control)

For each issue in `PROJECT_PLANNING.md`:

1. Click **"New Issue"**
2. Select the appropriate template
3. Copy content from planning doc
4. Set priority from the planning doc
5. Create the issue

The automation will:
- ✅ Auto-label based on title keywords
- ✅ Add to project board
- ✅ Place in appropriate column (if configured)

### Method 2: Bulk Creation Script (Advanced)

You can create a script to bulk-import issues. Here's a sample using GitHub CLI:

```bash
#!/bin/bash
# install: brew install gh
# auth: gh auth login

# Example: Create Issue #1
gh issue create \
  --title "[ALPHA] Project Architecture & Tech Stack Selection" \
  --label "alpha,critical,architecture" \
  --body-file issue_templates/issue_01.md
```

---

## 🎯 Suggested Workflow

### For Project Maintainers:

1. **Review Planning Doc**: `PROJECT_PLANNING.md`
2. **Create Alpha Issues First**: Issues #1-10
3. **Prioritize in Project Board**: Drag to order
4. **Assign to Milestones**: Create GitHub milestones
5. **Start Development**: Pick from top of Alpha column
6. **Move Cards**: As work progresses, move across columns
7. **Create Beta Issues**: Once alpha is stable

### For Contributors:

1. **Browse Project Board**: See what needs work
2. **Pick an Issue**: Comment "I'd like to work on this"
3. **Get Assigned**: Maintainer assigns you
4. **Submit PR**: Link to the issue
5. **Auto-closes**: PR merge auto-closes the issue

---

## 🔐 Required Permissions

The GitHub Actions workflows require these permissions:

- ✅ **`issues: write`** - To add labels (already included)
- ✅ **`GITHUB_TOKEN`** - Automatically provided (already configured)
- ⚠️ **Project Board Access** - Ensure Actions can access projects

### To verify project access:

1. Go to project settings
2. Under "Manage access"
3. Ensure "GitHub Actions" has write access

---

## 🚀 Advanced: Custom Automations

### Auto-move to "In Progress"

Add this to your project board automation (in project settings):

**When**: Issue is assigned
**Then**: Move to "🚧 In Progress"

### Auto-move to "Review"

**When**: PR is opened and linked to issue
**Then**: Move issue to "👀 Review"

### Auto-move to "Done"

**When**: PR is merged
**Then**: Move issue to "✅ Done"

### Auto-assign by label

**When**: Issue has label `critical`
**Then**: Assign to @Wikid82

---

## 📋 Creating Your First Issues

Here's a suggested order to create issues from the planning doc:

### Week 1 - Foundation (Create these first):
- [ ] Issue #1: Project Architecture & Tech Stack Selection
- [ ] Issue #2: Caddy Integration & Configuration Management
- [ ] Issue #3: Database Schema & Models

### Week 2 - Core UI:
- [ ] Issue #4: Basic Web UI Foundation
- [ ] Issue #5: Proxy Host Management (Core Feature)

### Week 3 - HTTPS & Security:
- [ ] Issue #6: Automatic HTTPS & Certificate Management
- [ ] Issue #7: User Authentication & Authorization

### Week 4 - Operations:
- [ ] Issue #8: Basic Access Logging
- [ ] Issue #9: Settings & Configuration UI
- [ ] Issue #10: Docker & Deployment Configuration

**Then**: Alpha release! 🎉

---

## 🎨 Project Board Views

Create multiple views for different perspectives:

### View 1: Kanban (Default)
All issues in status columns

### View 2: Priority Matrix
Group by: Priority
Sort by: Created date

### View 3: By Category
Group by: Labels (alpha, beta, etc.)
Filter: Not done

### View 4: This Sprint
Filter: Milestone = Current Sprint
Sort by: Priority

---

## 📱 Mobile & Desktop

The project board works great on:
- 💻 GitHub Desktop
- 📱 GitHub Mobile App
- 🌐 Web interface

You can triage issues from anywhere!

---

## 🆘 Troubleshooting

### Issue doesn't get labeled automatically
- Check title has bracketed keywords: `[ALPHA]`, `[CRITICAL]`
- Check workflow logs: Actions > Auto-label Issues
- Manually add labels - that's fine too!

### Issue doesn't appear on project board
- Check the workflow ran: Actions > Auto-add issues
- Verify your project URL in the workflow file
- Manually add to project from issue sidebar

### Labels not created
- Run the "Create Project Labels" workflow manually
- Check you have admin permissions
- Create labels manually from Issues > Labels

### Workflow permissions error
- Go to Settings > Actions > General
- Under "Workflow permissions"
- Select "Read and write permissions"
- Save

---

## 🎓 Learning Resources

- [GitHub Projects Docs](https://docs.github.com/en/issues/planning-and-tracking-with-projects)
- [GitHub Actions Docs](https://docs.github.com/en/actions)
- [Issue Templates](https://docs.github.com/en/communities/using-templates-to-encourage-useful-issues-and-pull-requests)

---

## ✅ Final Checklist

Before starting development, ensure:

- [ ] Project board created
- [ ] Project URL updated in workflow file
- [ ] Labels created (run the workflow)
- [ ] Issue templates tested
- [ ] First test issue created successfully
- [ ] Issue auto-labeled correctly
- [ ] Issue appeared on project board
- [ ] Column automation configured
- [ ] Team members invited to project
- [ ] Alpha milestone issues created (Issues #1-10)

---

## 🎉 You're Ready!

Your automated project management is set up! Every issue will now:
1. ✅ Automatically get labeled
2. ✅ Automatically added to project board
3. ✅ Move through columns as work progresses
4. ✅ Have structured templates for consistency

Focus on building awesome features - let automation handle the busywork! 🚀

---

**Questions?** Open an issue or discussion! The automation will handle it 😉
