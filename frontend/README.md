# Safe Ride Streamlit demo

This is a lightweight demo client for the Go API. It uses the existing ride,
driver, location, route, and stop endpoints; PostgreSQL and Valhalla remain
behind the API.

From the repository root, start the database and Boston routing service, then
run the Go API as described in the main [README](../README.md). In another
terminal:

```sh
python3 -m pip install -r frontend/requirements.txt
API_BASE_URL=http://localhost:8080 python3 -m streamlit run frontend/app.py
```

Open the Streamlit URL shown in the terminal (normally `http://localhost:8501`).
The default route examples use Boston coordinates. The demo uses the V1
caller-supplied rider and driver IDs; it does not add login sessions or replace
the backend's authorization model.
