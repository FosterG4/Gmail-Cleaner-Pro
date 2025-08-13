# Gmail Cleaner Pro

Gmail Cleaner Pro is a simple application that helps you clean up your Gmail inbox by automatically organizing emails into categories like Promotions, Social, Forums, and Updates. It connects securely to your Gmail account using Google's official authentication system.

## What This Application Does
- **Connects to Gmail**: Uses Google's login system
- **Organizes emails by type**: Automatically sorts Promotions, Social, Forums, and Updates
- **Batch cleaning**: Processes multiple emails at once for efficiency
- **Detailed logging**: Keeps track of all operations for your review
- **Easy-to-use interface**: Simple web page to control the cleaning process
- **API support**: Can be automated or integrated with other tools

## Prerequisites (Required Before Starting)

### ⚠️ IMPORTANT: Gmail API Must Be Enabled

**You MUST enable the Gmail API in Google Cloud Console before this application will work.** This is a mandatory step that cannot be skipped.

### What You Need:

1. **Go Programming Language**
   - Download and install Go version 1.21 or newer from [https://golang.org/dl/](https://golang.org/dl/)
   - After installation, open a command prompt and type `go version` to verify it's installed correctly

2. **Google Cloud Account** (Free)
   - You need a Google account (the same one you use for Gmail is fine)
   - Access to Google Cloud Console at [https://console.cloud.google.com/](https://console.cloud.google.com/)

3. **Gmail API Setup** (This is the most important step)
   - **Step 1**: Go to [Google Cloud Console](https://console.cloud.google.com/)
   - **Step 2**: Create a new project or select an existing one
   - **Step 3**: **ENABLE THE GMAIL API** - Search for "Gmail API" and click "Enable"
   - **Step 4**: Create OAuth2 credentials (detailed instructions below)

4. **Basic Computer Skills**
   - Ability to open a command prompt or terminal
   - Ability to copy and paste text
   - Ability to edit a text file

### Setting Up Google Cloud Console (Step-by-Step)

**This section is crucial - follow every step carefully:**

1. **Create a Google Cloud Project**
   - Go to [Google Cloud Console](https://console.cloud.google.com/)
   - Click "Select a project" at the top
   - Click "New Project"
   - Give it a name like "Gmail Cleaner Pro"
   - Click "Create"

2. **Enable the Gmail API** ⚠️ **CRITICAL STEP**
   - In your project, go to "APIs & Services" → "Library"
   - Search for "Gmail API"
   - Click on "Gmail API"
   - Click the blue "Enable" button
   - Wait for it to be enabled (this may take a minute)

3. **Create OAuth2 Credentials**
   - Go to "APIs & Services" → "Credentials"
   - Click "Create Credentials" → "OAuth 2.0 Client IDs"
   - If prompted, configure the OAuth consent screen first:
     - Choose "External" user type
     - Fill in the required fields (App name: "Gmail Cleaner Pro")
     - Add your email as a test user
   - For Application type, choose "Web application"
   - Name it "Gmail Cleaner Pro Client"
   - Under "Authorized redirect URIs", add: `http://localhost:8080/auth/callback`
   - Click "Create"
   - **IMPORTANT**: Copy the Client ID and Client Secret - you'll need these later

## How to Build the Application

Follow these steps in order:

### Step 1: Download the Code
- Download or clone this project to your computer
- Open a command prompt and navigate to the project folder

### Step 2: Install Dependencies
```bash
go mod tidy
```
*This command downloads all the required libraries*

### Step 3: Build the Application
```bash
go build -o bin/mailcleanerpro.exe ./cmd/server
```
*This creates the executable file you'll run*

### Step 4: Set Up Configuration
1. Copy the file `.env.sample` and rename it to `.env`
2. Open the `.env` file in a text editor
3. Replace the placeholder values with your actual Google Cloud credentials:
   ```
   GOOGLE_CLIENT_ID=your-actual-client-id-from-google-cloud.apps.googleusercontent.com
   GOOGLE_CLIENT_SECRET=your-actual-client-secret-from-google-cloud
   GOOGLE_REDIRECT_URL=http://localhost:8080/auth/callback
   PORT=8080
   ```

## How to Run the Application

### Step 1: Start the Server
```bash
./bin/mailcleanerpro.exe
```
*You should see messages indicating the server is running on port 8080*

### Step 2: Use the Application
1. Open your web browser and go to: `http://localhost:8080/`
2. Click the "Connect Gmail Account" button
3. Sign in with your Google account when prompted
4. Grant permission for the app to access your Gmail
5. You'll be redirected back to the application
6. Select which email categories you want to clean
7. Set how many emails to process per category (start with a small number like 10 for testing)
8. Click "Clean Now"
9. Review the results

## Troubleshooting

### Common Issues:

**"Gmail API has not been used" Error**
- You forgot to enable the Gmail API in Google Cloud Console
- Go back to the Prerequisites section and follow the Gmail API setup steps

**"Invalid Client" Error**
- Your Client ID or Client Secret is wrong
- Double-check your `.env` file has the correct values from Google Cloud Console

**"Redirect URI Mismatch" Error**
- The redirect URI in Google Cloud Console doesn't match
- Make sure you added exactly: `http://localhost:8080/auth/callback`

**Application Won't Start**
- Make sure Go is installed correctly (`go version` should work)
- Make sure you ran `go mod tidy` and `go build` successfully
- Check that port 8080 isn't being used by another application

## Development Mode (Optional)

If you're a developer and want to make changes to the code:

```bash
# Install Air for automatic reloading when you change code
go install github.com/cosmtrek/air@latest

# Run in development mode (automatically restarts when you change files)
air
```

## API Usage (For Developers)

If you want to integrate this application with other tools, you can use the API endpoints:

### Authentication for API Usage
For API integration, you'll need to implement OAuth2 flow or use the web interface to authenticate:
1. Use the web interface at `http://localhost:8080/` to connect your Gmail account
2. The application handles the OAuth2 flow automatically
3. For direct API access, implement OAuth2 client credentials flow

### Clean Emails via API
```bash
POST http://localhost:8080/api/clean
Content-Type: application/json
X-Access-Token: <your-access-token>

{
  "categories": ["CATEGORY_PROMOTIONS", "CATEGORY_SOCIAL"],
  "max_per_category": 100,
  "action": "trash"
}
```

### Check Status
```bash
GET http://localhost:8080/api/status
X-Access-Token: <your-access-token>
```

**Note:** For API usage, you can obtain the access token by authenticating through the web interface first, then extracting it from the browser's local storage or implementing your own OAuth2 flow.

**⚠️ Security Note:** Never commit your `.env` file to version control. The `.env.sample` file is provided as a template with example values only.

## Project Structure

```
mailcleanerpro/
├── cmd/server/          # Main application
├── internal/            # Internal application code
├── pkg/                 # Reusable packages
├── web/                 # Web interface files
├── bin/                 # Built executable (created after build)
├── .env                 # Your configuration file (you create this)
└── README.md            # This documentation
```

## Security & Privacy

- **Your data stays private**: The application only accesses your Gmail through Google's secure API
- **No data storage**: Email content is never stored on your computer
- **Secure authentication**: Uses Google's OAuth2 system (the same login system Gmail uses)
- **Audit trail**: All operations are logged so you can see exactly what happened
- **Minimal permissions**: Only requests the minimum access needed to clean emails

## What's Coming Next

- **Better filtering**: More options for which emails to clean
- **Scheduled cleaning**: Set it to run automatically
- **Preview before delete**: See what will be deleted before it happens

## Support

If you have problems:
1. Check the Troubleshooting section above
2. Make sure you followed all the Prerequisites steps
3. Verify your Google Cloud Console setup is correct
4. Check that the Gmail API is enabled

For additional help, please open an issue on the project repository.

## License

This project is open source and available under the [MIT License](LICENSE).
