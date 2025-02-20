# Veracy Server

Veracy Server is a distributed content management system that handles image storage, user authentication, and payment verification using the Arweave blockchain.

## Features

- **Distributed Storage**: P2P network for distributed image storage and retrieval
- **Blockchain Integration**: Arweave blockchain integration for payment verification and content ownership
- **User Authentication**: JWT-based authentication system with public key cryptography
- **Access Control**: Content privacy management with payment verification
- **Admin Interface**: Administrative tools for content moderation
- **Feedback System**: User feedback collection and management

## Prerequisites

- Go 1.19 or higher
- SQLite3
- Access to Arweave network
- SSL certificate (for production)

## Installation

1. Clone the repository:

```bash
git clone https://github.com/yourusername/veracy.server.git
cd veracy.server
```

2. Install dependencies:

```bash
go mod tidy
```

3. Configure environment variables:

```bash
# .env
SECRET=your_jwt_secret_here
ADMIN_KEY=your_admin_rsa_public_key_here
```

4. Start the server:

```bash
go run .
```

5. Bootstrap the server:

```bash
# TCP:
go run . -b /ip4/{SERVER_IP}/tcp/8079/p2p/{NODE_ID} -p {PORT} -g {GROUP_TOPIC}

# UDP QUIC:
go run . -b /ip4/{SERVER_IP}/udp/8078/quic-v1/p2p/{NODE_ID} -p {PORT} -g {GROUP_TOPIC}
```

## Configuration

The server can be configured using command-line flags:

- `-p`: HTTP interface port (default: 8080)
- `-p-tcp`: TCP port for P2P network (default: 8079)
- `-p-udp`: UDP port for P2P network (default: 8078)
- `-b`: Bootstrap node multiaddress
- `-g`: P2P network group topic

## Architecture

### Components
- **Distributed Node**: P2P network for content distribution
- **Database**: SQLite storage for user data and content metadata
- **Arweave Integration**: Blockchain payment verification
- **Authentication System**: Public key and challenge-based authentication

### Security Features
- JWT-based session management
- Public key cryptography for user authentication
- Content access control with payment verification
- SSL/TLS encryption support
