################################
# STEP 1 build executable binary
################################
FROM mcr.microsoft.com/dotnet/sdk:6.0 AS builder

RUN mkdir /app
COPY . /app
WORKDIR /app

RUN dotnet dev-certs https --trust
RUN dotnet build

EXPOSE 7032 5007

CMD ["dotnet", "run"]

################################
# STEP 2 build final container
################################
#FROM mcr.microsoft.com/dotnet/aspnet:6.0
#
#COPY --from=builder /app/bin/Debug/net6.0/MyFirstApp.dll /app/bin/Debug/net6.0/MyFirstApp.dll
#
#EXPOSE 8443
#
#CMD ["dotnet", "run"]
