FROM alpine:3.21 AS app

WORKDIR /app
COPY build/lua-spider ./lua-spider
COPY configs ./configs
COPY scripts ./scripts

EXPOSE 8080
CMD ["./lua-spider"]

FROM alpine:3.21 AS benchmark-mock

COPY build/benchmark-mock /benchmark-mock

EXPOSE 8080
CMD ["/benchmark-mock"]
