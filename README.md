# CineSeer

CineSeer is a Go web server application that provides information about TV series, including both current and upcoming shows. It offers a clean API interface and caches images for improved performance.

## Features

- RESTful API endpoints for current and upcoming TV series
- Image caching system for optimized performance
- CORS support for cross-origin requests
- Request logging with detailed information
- Response caching with 15-minute expiration
- HTML template rendering for the frontend
- Static file serving

## Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd CineSeer
```

2. Install dependencies:
```bash
go mod download
```

3. Create a `.env` file in the project root (see Environment Variables section)

## Environment Variables

Create a `.env` file in the project root with the following variables:
```
# Add your environment variables here
```

## Usage

1. Start the server:
```bash
go run .
```

The server will start on `http://localhost:3001`

### API Endpoints

- `GET /` - Main page with HTML template
- `GET /api/series` - Get list of current TV series
- `GET /api/upcoming-series` - Get list of upcoming TV series

### Static Files

Static files (including cached images) are served from the `/static` directory and accessible via `/static/*` routes.

## Development

### Prerequisites

- Go 1.22 or higher
- [Fiber v2](https://github.com/gofiber/fiber)

### Project Structure

- `server.go` - Main server setup and route handlers
- `new_series.go` - Current series data handling
- `upcoming_series.go` - Upcoming series data handling
- `cache.go` - Image caching functionality
- `/static/cache/` - Cached images directory
- `/views/` - HTML templates

### Dependencies

- `github.com/gofiber/fiber/v2` - Web framework
- `github.com/gofiber/template/html/v2` - HTML template engine
- `github.com/joho/godotenv` - Environment variable management

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.
