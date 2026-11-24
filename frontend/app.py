"""Streamlit demo client for the Safe Ride Go API."""

from __future__ import annotations

import os
from statistics import mean
from typing import Any

import pydeck as pdk
import requests
import streamlit as st


DEFAULT_API_URL = os.getenv("API_BASE_URL", "http://localhost:8080").rstrip("/")
BOSTON_COMMON = (42.3551, -71.0656)
FENWAY = (42.3429, -71.1003)


class APIError(Exception):
    """Readable error returned by the Go API or HTTP client."""


def api_request(method: str, path: str, *, params: dict[str, Any] | None = None,
                payload: dict[str, Any] | None = None) -> dict[str, Any]:
    base_url = st.session_state.get("api_base_url", DEFAULT_API_URL).rstrip("/")
    try:
        response = requests.request(
            method,
            f"{base_url}{path}",
            params=params,
            json=payload,
            timeout=30,
        )
    except requests.RequestException as exc:
        raise APIError(f"Could not reach the Go API at {base_url}: {exc}") from exc

    try:
        body = response.json()
    except ValueError:
        body = {"message": response.text or response.reason}
    if not response.ok:
        message = body.get("message") or body.get("error") or response.reason
        code = body.get("error")
        prefix = f"{code}: " if code else ""
        raise APIError(f"{prefix}{message}")
    if not isinstance(body, dict):
        raise APIError("The API returned an unexpected response format.")
    return body


def decode_polyline(encoded: str, precision: int = 6) -> list[list[float]]:
    """Decode a Valhalla encoded polyline into deck.gl [longitude, latitude] points."""
    points: list[list[float]] = []
    latitude = longitude = index = 0
    factor = 10**precision
    while index < len(encoded):
        values: list[int] = []
        for _ in range(2):
            result = shift = 0
            while True:
                if index >= len(encoded):
                    return points
                byte = ord(encoded[index]) - 63
                index += 1
                result |= (byte & 0x1F) << shift
                shift += 5
                if byte < 0x20:
                    break
            values.append(~(result >> 1) if result & 1 else result >> 1)
        latitude += values[0]
        longitude += values[1]
        points.append([longitude / factor, latitude / factor])
    return points


def show_api_error(error: APIError) -> None:
    st.error(str(error))


def format_eta(seconds: float) -> str:
    minutes = max(1, round(seconds / 60))
    return f"{minutes} min"


def route_map(route: dict[str, Any], pickup: tuple[float, float],
              destination: tuple[float, float]) -> None:
    path: list[list[float]] = []
    for leg in route.get("legs", []):
        path.extend(decode_polyline(leg.get("shape", "")))
    if len(path) < 2:
        st.info("The route was returned, but its map geometry could not be displayed.")
        return

    midpoint = [mean(point[1] for point in path), mean(point[0] for point in path)]
    layers = [
        pdk.Layer(
            "PathLayer",
            data=[{"path": path}],
            get_path="path",
            get_color=[71, 232, 183] if route.get("safe_mode_used") else [113, 174, 255],
            get_width=7,
            width_min_pixels=4,
            pickable=False,
        ),
        pdk.Layer(
            "ScatterplotLayer",
            data=[
                {"position": [pickup[1], pickup[0]], "label": "Pickup"},
                {"position": [destination[1], destination[0]], "label": "Drop-off"},
            ],
            get_position="position",
            get_fill_color=[249, 193, 92],
            get_radius=85,
            radius_min_pixels=6,
            pickable=True,
        ),
    ]
    st.pydeck_chart(
        pdk.Deck(
            layers=layers,
            initial_view_state=pdk.ViewState(
                latitude=midpoint[0], longitude=midpoint[1], zoom=12.5,
                pitch=35, bearing=0,
            ),
            map_style="https://basemaps.cartocdn.com/gl/dark-matter-gl-style/style.json",
            tooltip={"text": "{label}"},
        ),
        use_container_width=True,
    )


def show_route(route: dict[str, Any], pickup: tuple[float, float],
               destination: tuple[float, float]) -> None:
    left, middle, right = st.columns(3)
    left.metric("Estimated drive", format_eta(route.get("duration_seconds", 0)))
    middle.metric("Road distance", f"{route.get('distance_km', 0):.1f} km")
    mode_label = "Lower reported risk" if route.get("safe_mode_used") else "Fastest route"
    right.metric("Route mode", mode_label)

    if route.get("safe_mode_used"):
        st.success("Selected a route with lower historical reported-incident risk within the 20% travel-time limit.")
        st.caption("This is based on available Boston police reports; it is not a guarantee of personal safety.")
    elif route.get("fallback"):
        st.warning(route.get("fallback_reason") or "No lower-risk alternative fit the travel-time limit; showing the normal route.")
    elif route.get("mode") == "NORMAL_FALLBACK":
        st.warning(route.get("fallback_reason") or "Safe Mode could not find a supported route, so the normal route is shown.")

    route_map(route, pickup, destination)


def rider_tab() -> None:
    st.markdown('<div class="eyebrow">RIDER EXPERIENCE</div>', unsafe_allow_html=True)
    st.subheader("Start with where you’re going")
    st.write("Request a Boston ride, compare route modes, and keep the trip details together.")

    with st.expander("Create a rider account", expanded=not st.session_state.get("rider_id")):
        with st.form("register_rider"):
            name = st.text_input("Name", placeholder="Alex Morgan")
            phone = st.text_input("Phone", placeholder="5550100123")
            password = st.text_input("Password", type="password")
            submitted = st.form_submit_button("Create rider account", type="primary")
        if submitted:
            try:
                user = api_request("POST", "/users", payload={"name": name, "phone": phone, "password": password})
                st.session_state.rider_id = user["user_id"]
                st.success(f"Rider profile ready · ID {user['user_id']}")
            except (APIError, KeyError) as error:
                show_api_error(error if isinstance(error, APIError) else APIError("The API did not return a rider ID."))

    current_rider_id = int(st.session_state.get("rider_id", 0) or 0)
    with st.form("request_ride"):
        rider_id = st.number_input("Rider ID", min_value=0, step=1, value=current_rider_id,
                                   help="Use the ID returned when you create a rider profile.")
        st.markdown("#### Pickup")
        p1, p2 = st.columns(2)
        pickup_lat = p1.number_input("Latitude", value=BOSTON_COMMON[0], format="%.6f", key="pickup_lat")
        pickup_lon = p2.number_input("Longitude", value=BOSTON_COMMON[1], format="%.6f", key="pickup_lon")
        st.caption("Default: Boston Common")
        st.markdown("#### Drop-off")
        d1, d2 = st.columns(2)
        destination_lat = d1.number_input("Latitude", value=FENWAY[0], format="%.6f", key="destination_lat")
        destination_lon = d2.number_input("Longitude", value=FENWAY[1], format="%.6f", key="destination_lon")
        st.caption("Default: Fenway Park")
        safe_mode = st.toggle("Prefer lower historical reported-incident risk", value=False)
        request_submitted = st.form_submit_button("Find my route", type="primary", use_container_width=True)

    if request_submitted:
        if rider_id <= 0:
            st.warning("Create a rider account or enter an existing rider ID first.")
        else:
            pickup = (pickup_lat, pickup_lon)
            destination = (destination_lat, destination_lon)
            try:
                ride = api_request("POST", "/rides", payload={
                    "user_id": int(rider_id),
                    "pickup": {"latitude": pickup[0], "longitude": pickup[1]},
                    "destination": {"latitude": destination[0], "longitude": destination[1]},
                    "safe_mode": safe_mode,
                })
                st.session_state.rider_id = int(rider_id)
                st.session_state.ride_id = ride["ride_id"]
                st.session_state.route_points = (pickup, destination)
                st.success(f"Ride {ride['ride_id']} requested · status {ride['status']}")
                route = api_request("GET", f"/rides/{ride['ride_id']}/route")
                st.session_state.route_result = route
            except (APIError, KeyError) as error:
                show_api_error(error if isinstance(error, APIError) else APIError("The API response was missing a required ID."))

    ride_id = st.session_state.get("ride_id")
    if ride_id:
        st.divider()
        st.markdown(f"#### Your latest trip · #{ride_id}")
        try:
            ride = api_request("GET", f"/rides/{ride_id}")
            st.caption(f"Ride status: **{ride.get('status', 'UNKNOWN')}**")
        except APIError as error:
            show_api_error(error)

        refresh_col, info_col = st.columns([1, 2])
        if refresh_col.button("Refresh route", use_container_width=True):
            try:
                st.session_state.route_result = api_request("GET", f"/rides/{ride_id}/route")
            except APIError as error:
                show_api_error(error)
        info_col.caption("Route geometry and ETA come from Valhalla through the Go API.")
        route = st.session_state.get("route_result")
        points = st.session_state.get("route_points")
        if route and points:
            show_route(route, points[0], points[1])


def driver_tab() -> None:
    st.markdown('<div class="eyebrow">DRIVER CONSOLE</div>', unsafe_allow_html=True)
    st.subheader("Keep the trip moving")
    st.write("Register a driver, share a location ping, and update the ride as it progresses.")

    with st.expander("Register a driver", expanded=not st.session_state.get("driver_id")):
        with st.form("register_driver"):
            user_id = st.number_input("Linked user ID", min_value=0, step=1,
                                      value=int(st.session_state.get("rider_id", 0) or 0))
            vehicle_type = st.selectbox("Vehicle", ["sedan", "SUV", "hybrid", "other"])
            plate = st.text_input("Plate number", placeholder="BOS-1234")
            submitted = st.form_submit_button("Register driver", type="primary")
        if submitted:
            try:
                driver = api_request("POST", "/drivers", payload={
                    "user_id": int(user_id), "vehicle_type": vehicle_type, "plate_number": plate,
                })
                st.session_state.driver_id = driver["driver_id"]
                st.success(f"Driver profile ready · ID {driver['driver_id']}")
            except (APIError, KeyError) as error:
                show_api_error(error if isinstance(error, APIError) else APIError("The API did not return a driver ID."))

    st.markdown("#### Current location")
    location1, location2 = st.columns(2)
    driver_lat = location1.number_input("Latitude", value=BOSTON_COMMON[0], format="%.6f", key="driver_lat")
    driver_lon = location2.number_input("Longitude", value=BOSTON_COMMON[1], format="%.6f", key="driver_lon")
    driver_id = int(st.session_state.get("driver_id", 0) or 0)
    location_col, lookup_col = st.columns(2)
    if location_col.button("Send location ping", disabled=driver_id <= 0, use_container_width=True):
        try:
            result = api_request("POST", f"/drivers/{driver_id}/location", payload={"lat": driver_lat, "lng": driver_lon})
            st.success(result.get("message", "Location updated."))
            if result.get("completed_stop_id"):
                st.info(f"Stop {result['completed_stop_id']} marked complete from this location ping.")
        except APIError as error:
            show_api_error(error)
    if lookup_col.button("Find nearby drivers", use_container_width=True):
        try:
            result = api_request("GET", "/drivers/available", params={"lat": driver_lat, "long": driver_lon})
            st.write(f"{len(result.get('drivers', []))} available within 2 km")
            st.dataframe(result.get("drivers", []), use_container_width=True, hide_index=True)
        except APIError as error:
            show_api_error(error)

    st.divider()
    st.markdown("#### Ride controls")
    ride_col, driver_col = st.columns(2)
    ride_id = ride_col.number_input("Ride ID", min_value=0, step=1,
                                    value=int(st.session_state.get("ride_id", 0) or 0))
    control_driver_id = driver_col.number_input("Driver ID", min_value=0, step=1, value=driver_id)
    controls = st.columns(4)
    actions = [
        ("Accept", "accept", {"driver_id": int(control_driver_id)}),
        ("Start", "start", {"driver_id": int(control_driver_id)}),
        ("Complete", "complete", {"driver_id": int(control_driver_id)}),
        ("Cancel", "cancel", {"driver_id": int(control_driver_id)}),
    ]
    for column, (label, action, payload) in zip(controls, actions):
        if column.button(label, disabled=ride_id <= 0 or control_driver_id <= 0,
                         use_container_width=True, key=f"ride_{action}"):
            try:
                result = api_request("POST", f"/rides/{int(ride_id)}/{action}", payload=payload)
                st.success(result.get("message", f"Ride {action} request succeeded."))
            except APIError as error:
                show_api_error(error)

    with st.expander("Add an optional stop"):
        st.caption("Available during an ongoing ride. The server checks route detour limits and assigns stop order.")
        stop1, stop2 = st.columns(2)
        stop_lat = stop1.number_input("Stop latitude", value=42.3557, format="%.6f", key="stop_lat")
        stop_lon = stop2.number_input("Stop longitude", value=-71.0648, format="%.6f", key="stop_lon")
        stop_rider_id = st.number_input("Rider ID for this ride", min_value=0, step=1,
                                        value=int(st.session_state.get("rider_id", 0) or 0), key="stop_rider_id")
        if st.button("Add stop", disabled=ride_id <= 0 or stop_rider_id <= 0):
            try:
                result = api_request("POST", f"/rides/{int(ride_id)}/stops", payload={
                    "rider_id": int(stop_rider_id), "location": {"lat": stop_lat, "lng": stop_lon},
                })
                st.success(f"Stop {result['sequence_order']} added to ride {result['ride_id']}.")
            except (APIError, KeyError) as error:
                show_api_error(error if isinstance(error, APIError) else APIError("The API returned an unexpected stop response."))


def main() -> None:
    st.set_page_config(page_title="Safe Ride · Boston", page_icon="🚘", layout="wide")
    if "api_base_url" not in st.session_state:
        st.session_state.api_base_url = DEFAULT_API_URL

    st.markdown(
        """
        <style>
        @import url('https://fonts.googleapis.com/css2?family=DM+Mono:wght@400;500&family=Manrope:wght@400;500;600;700;800&display=swap');
        :root { --ink:#edf4f1; --muted:#9aaea8; --mint:#47e8b7; --line:rgba(177,208,197,.14); }
        html, body, [class*="css"] { font-family: 'Manrope', sans-serif; }
        .stApp { background: radial-gradient(ellipse at 72% 0%, #17322e 0%, #0b1517 46%, #091113 100%); color:var(--ink); }
        [data-testid="stHeader"] { background:transparent; }
        [data-testid="stSidebar"] { background:#0d191b; border-right:1px solid var(--line); }
        .block-container { max-width:1240px; padding-top:2.1rem; padding-bottom:4rem; }
        h1,h2,h3 { color:#f0f7f4; letter-spacing:-.045em; }
        h1 { font-weight:800; }
        p, label, [data-testid="stCaptionContainer"] { color:#a8bbb5; }
        .hero { padding:2.1rem 2.2rem; border:1px solid var(--line); border-radius:24px;
          background:linear-gradient(120deg,rgba(29,57,52,.94),rgba(17,32,34,.74));
          box-shadow:0 24px 80px rgba(0,0,0,.22); margin-bottom:1.6rem; }
        .hero-title { font-size:clamp(2.4rem,5vw,4.4rem); line-height:.98; font-weight:800; color:#f3faf7; letter-spacing:-.075em; }
        .hero-title span { color:var(--mint); }
        .hero-copy { font-size:1rem; max-width:620px; color:#a9bdb6; margin-top:1rem; }
        .eyebrow { color:var(--mint); font-family:'DM Mono',monospace; font-size:.72rem; letter-spacing:.18em; margin-bottom:.5rem; }
        [data-testid="stMetric"] { background:rgba(21,40,41,.72); border:1px solid var(--line); padding:1rem 1.15rem; border-radius:16px; }
        [data-testid="stMetricValue"] { color:#f2faf7; font-weight:800; }
        [data-testid="stMetricLabel"] { color:#9db2aa; }
        [data-testid="stTabs"] button { font-weight:700; }
        [data-testid="stTabs"] button[aria-selected="true"] { color:var(--mint); }
        .stButton button[kind="primary"], .stFormSubmitButton button[kind="primary"] { background:#42dcae; border:0; color:#062019; font-weight:800; }
        .stButton button { border-radius:12px; }
        [data-testid="stExpander"] { border:1px solid var(--line); border-radius:14px; background:rgba(17,32,34,.55); }
        [data-testid="stAlert"] { border-radius:14px; }
        </style>
        <div class="hero">
          <div class="eyebrow">BOSTON · A DIFFERENT WAY THROUGH THE CITY</div>
          <div class="hero-title">Your ride,<br><span>with more context.</span></div>
          <div class="hero-copy">A ride-hailing prototype that pairs practical trip tools with optional routing informed by historical reported incidents.</div>
        </div>
        """,
        unsafe_allow_html=True,
    )

    with st.sidebar:
        st.markdown("### Safe Ride")
        st.caption("BOSTON PILOT · V1 DEMO")
        api_url = st.text_input("Go API URL", value=st.session_state.api_base_url)
        st.session_state.api_base_url = api_url.strip().rstrip("/") or DEFAULT_API_URL
        try:
            health = api_request("GET", "/health")
            st.success(f"API online · database {health.get('database', 'unknown')}")
        except APIError as error:
            st.error("API offline")
            st.caption(str(error))
        st.divider()
        st.caption("Safe Mode is opt-in and uses Boston reported-incident data where route coverage is available.")

    rider, driver = st.tabs(["🚘  Book a ride", "⌁  Driver console"])
    with rider:
        rider_tab()
    with driver:
        driver_tab()


if __name__ == "__main__":
    main()
