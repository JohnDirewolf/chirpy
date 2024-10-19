Chirpy
A server that provides basic messaging service.

Created as part of Boot.dev Back-End Programming  

Installation Instructions:
This program requires Postgresql.
Also you may need "github.com/joho/godotenv"

You will need to create an .env file:
DB_URL="connection string to your database"
SECRET="secret key for authorizations"
POLKA_KEY="apikey the program should use for mock user upgrade webhooks"


Clone the Repository:
    First, navigate to the repository page on GitHub.
    Use the "Code" button to reveal the options, and copy the URL.
    Open your terminal, and run the following command to clone the repository to your local machine:
    git clone https://github.com/JohnDirewolf/chirpy.git

Navigate into the Project Directory:
    Change directory to the newly cloned project's folder:
    cd chirpy

Set Up the Environment (if applicable):
    Create a virtual environment to manage dependencies:
    python -m venv myenv

Activate the Virtual Environment:

For Windows:
.\myenv\Scripts\activate
For macOS and Linux:
source myenv/bin/activate

Install Dependencies:
    With your virtual environment activated, install the required dependencies