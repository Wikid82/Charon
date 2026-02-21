#!/bin/bash

# This script updates npm dependencies for the project.

cd /projects/Charon || exit

echo "Updating root npm dependencies..."

npm update
npm dedup
npm audit --audit-level=high
npm audit fix
npm outdated
npm install

echo "Root npm dependencies updated successfully."

cd /projects/Charon/frontend || exit

echo "Updating frontend npm dependencies..."

npm update
npm dedup
npm audit --audit-level=high
npm audit fix
npm outdated
npm install

echo "Frontend npm dependencies updated successfully."
