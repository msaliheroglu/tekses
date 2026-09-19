# Panel bağlamıyla derlenir: docker build apps/moderator
# NEXT_PUBLIC_* değişkenleri istemci paketine DERLEME anında gömülür;
# QR'ların telefondan erişilen adresi build arg olarak verilmelidir.
FROM node:22-alpine AS build
WORKDIR /app
ARG NEXT_PUBLIC_GATEWAY_PUBLIC_URL=http://localhost:8080
ENV NEXT_PUBLIC_GATEWAY_PUBLIC_URL=$NEXT_PUBLIC_GATEWAY_PUBLIC_URL
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:22-alpine
WORKDIR /app
ENV NODE_ENV=production HOSTNAME=0.0.0.0 PORT=3000
COPY --from=build /app/.next/standalone ./
COPY --from=build /app/.next/static ./.next/static
EXPOSE 3000
CMD ["node", "server.js"]
