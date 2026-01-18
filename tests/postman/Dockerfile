FROM node:18-alpine

WORKDIR /app

COPY package*.json ./

# Prefer deterministic installs in CI if package-lock.json is present
RUN if [ -f package-lock.json ]; then npm ci; else npm install; fi

COPY . .

ENTRYPOINT ["node", "run-tests.js"]
CMD ["--environment", "docker"]