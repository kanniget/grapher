# frontend build
FROM node:20 AS frontend
WORKDIR /frontend
COPY frontend/package.json frontend/package-lock.json frontend/vite.config.js frontend/index.html ./
COPY frontend/src ./src
RUN npm install && npm run build

# backend build
FROM golang:1.23 AS backend
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend ./backend
RUN cd backend && go build -o /server && go build -o /dbtool ./cmd/dbtool

# final image
FROM debian:stable-slim
WORKDIR /app
COPY --from=backend /server ./server
COPY --from=backend /dbtool ./dbtool
COPY --from=frontend /backend/public ./public
COPY config.json ./config.json
EXPOSE 8080
CMD ["./server"]
