# Multi-stage build for smaller final image
FROM node:20-alpine AS builder

WORKDIR /app

# Copy package files
COPY package*.json ./

# Install dependencies
RUN npm ci

# Copy source code
COPY . .

# Build the application
RUN npm run build

# Final stage - serve with nginx
FROM nginx:alpine

# Install gettext for envsubst
RUN apk add --no-cache gettext

# Copy built assets from builder stage
COPY --from=builder /app/dist /usr/share/nginx/html

# Copy the template file to nginx html directory
COPY --from=builder /app/public/auth.template.json /usr/share/nginx/html/auth.template.json

# Copy nginx configuration
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Create nginx user
RUN chown -R nginx:nginx /usr/share/nginx/html && \
    chown -R nginx:nginx /var/cache/nginx && \
    chown -R nginx:nginx /var/log/nginx && \
    chown -R nginx:nginx /etc/nginx/conf.d && \
    touch /var/run/nginx.pid && \
    chown -R nginx:nginx /var/run/nginx.pid

# Switch to non-root user
USER nginx

# Expose port
EXPOSE 3000

# Use envsubst to populate auth.json at runtime, then start nginx
CMD ["/bin/sh", "-c", "envsubst < /usr/share/nginx/html/auth.template.json > /usr/share/nginx/html/auth.json && exec nginx -g 'daemon off;'"]

