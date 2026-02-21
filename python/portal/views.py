"""View layer for the PbT prototype."""

import math
import random
import re
from io import BytesIO

from django.contrib import messages
from django.contrib.auth import authenticate, login, logout
from django.contrib.auth.decorators import login_required
from django.contrib.auth.models import User
from django.http import HttpResponse
from django.shortcuts import get_object_or_404, redirect, render
from django.urls import reverse
from PIL import Image, ImageDraw

from .models import ContactRequest, MemberProfile, TttResult
from .ttt import QUESTIONS, evaluate_answers

GPS_RE = re.compile(r"^\d+(?:\.\d+)?\s*[NnSs]\s*,?\s*\d+(?:\.\d+)?\s*[EeWw]$")


def _parse_gps(value):
    if not value:
        return None
    clean = value.replace(",", " ").split()
    if len(clean) != 4:
        return None
    lat = float(clean[0]) * (-1 if clean[1].upper() == "S" else 1)
    lon = float(clean[2]) * (-1 if clean[3].upper() == "W" else 1)
    return lat, lon


def _status(viewer, other):
    if viewer == other:
        return "self"
    ab = ContactRequest.objects.filter(from_member=viewer, to_member=other).exists()
    ba = ContactRequest.objects.filter(from_member=other, to_member=viewer).exists()
    if ab and ba:
        return "in_contact"
    if ab:
        return "RCD_sent"
    if ba:
        return "RCD_received"
    return "no_contact"


def _distance_km(user_a, user_b):
    a = _parse_gps(user_a.memberprofile.gps_coordinates)
    b = _parse_gps(user_b.memberprofile.gps_coordinates)
    if not a or not b:
        return None
    d = math.sqrt((a[0] - b[0]) ** 2 + (a[1] - b[1]) ** 2)
    return d * 100.0


def home(request):
    if request.user.is_authenticated:
        return redirect("status", username=request.user.username)
    return render(request, "portal/home.html")


def register_view(request):
    if request.method == "POST":
        username = request.POST.get("username", "").strip()
        password = request.POST.get("password", "")
        if User.objects.filter(username=username).exists():
            messages.error(request, "Username is already in use.")
            return render(request, "portal/register.html")
        user = User.objects.create_user(
            username=username,
            password=password,
            first_name=request.POST.get("fullname", "").strip(),
            email=request.POST.get("email", "").strip(),
        )
        gps = request.POST.get("gps_coordinates", "").strip()
        if gps and not GPS_RE.match(gps):
            user.delete()
            messages.error(request, "GPS coordinates must look like: 52.5N, 13.4E")
            return render(request, "portal/register.html")
        MemberProfile.objects.create(
            user=user,
            town=request.POST.get("town", "").strip(),
            country=request.POST.get("country", "").strip(),
            motto1=request.POST.get("motto1", "").strip(),
            motto2=request.POST.get("motto2", "").strip(),
            likes=request.POST.get("likes", "").strip(),
            dislikes=request.POST.get("dislikes", "").strip(),
            gps_coordinates=gps,
            enneagramtype1=int(request.POST.get("enneagramtype1", 0) or 0),
            enneagramtype2=int(request.POST.get("enneagramtype2", 10) or 10),
        )
        login(request, user)
        return redirect("ttt")
    return render(request, "portal/register.html")


def login_view(request):
    if request.method == "POST":
        user = authenticate(
            request,
            username=request.POST.get("username", ""),
            password=request.POST.get("password", ""),
        )
        if user:
            login(request, user)
            return redirect("status", username=user.username)
        messages.error(request, "Login failed.")
    return render(request, "portal/login.html")


@login_required
def logout_view(request):
    logout(request)
    return redirect("home")


@login_required
def ttt_view(request):
    if request.method == "POST":
        answers = {}
        for i, q in enumerate(QUESTIONS):
            value = request.POST.get(f"q{i}", "")
            allowed = {q[1][0], q[2][0], ""}
            answers[i] = value if value in allowed else ""
        result = evaluate_answers(answers)
        if not result:
            messages.error(request, "Please answer at least five questions per MBTI dimension.")
            return render(request, "portal/ttt.html", {"questions": list(enumerate(QUESTIONS))})
        TttResult.objects.update_or_create(user=request.user, defaults=result)
        return render(request, "portal/ttt_result.html", {"result": result})
    return render(request, "portal/ttt.html", {"questions": list(enumerate(QUESTIONS))})


def _annotate_members(viewer, members):
    rows = []
    for m in members:
        ttt = TttResult.objects.filter(user=m).first()
        rows.append(
            {
                "member": m,
                "profile": m.memberprofile,
                "ttt": ttt,
                "status": _status(viewer, m),
                "can_send": _status(viewer, m) in {"no_contact", "RCD_received"},
            }
        )
    return rows


@login_required
def search_view(request):
    members = None
    error = None
    x_var = "ei"
    y_var = "sn"
    if request.method == "POST":
        x_var = request.POST.get("x_var", "ei")
        y_var = request.POST.get("y_var", "sn")
        if not any(request.POST.get(x) for x in ["motto_contains", "ttt_types", "only_not_contacts", "in_my_country", "max_km"]):
            error = "Select at least one filter."
        else:
            qs = User.objects.exclude(id=request.user.id).select_related("memberprofile")
            if request.POST.get("only_not_contacts"):
                qs = [u for u in qs if _status(request.user, u) == "no_contact"]
            else:
                qs = list(qs)
            if request.POST.get("in_my_country"):
                qs = [u for u in qs if u.memberprofile.country == request.user.memberprofile.country]
            motto_contains = request.POST.get("motto_contains", "").strip().lower()
            if motto_contains:
                qs = [u for u in qs if motto_contains in u.memberprofile.motto1.lower() or motto_contains in u.memberprofile.motto2.lower()]
            ttt_types = [x.strip().upper() for x in request.POST.get("ttt_types", "").split(",") if x.strip()]
            if ttt_types:
                users_by_type = set(TttResult.objects.filter(ttt_type__in=ttt_types).values_list("user_id", flat=True))
                qs = [u for u in qs if u.id in users_by_type]
            max_km = request.POST.get("max_km", "").strip()
            if max_km.isdigit() and int(max_km) > 0:
                limit = int(max_km)
                qs = [u for u in qs if (_distance_km(request.user, u) or 10**9) <= limit]
            members = _annotate_members(request.user, qs)
    return render(request, "portal/search.html", {"members": members, "error": error, "x_var": x_var, "y_var": y_var})


@login_required
def send_rcd_view(request):
    usernames = request.POST.getlist("members")
    action = request.POST.get("action", "send")
    for username in usernames:
        other = User.objects.filter(username=username).first()
        if not other or other == request.user:
            continue
        status = _status(request.user, other)
        if action == "send" and status in {"no_contact", "RCD_received"}:
            ContactRequest.objects.get_or_create(from_member=request.user, to_member=other)
        elif action == "accept" and status == "RCD_received":
            ContactRequest.objects.get_or_create(from_member=request.user, to_member=other)
        elif action == "reject" and status == "RCD_received":
            ContactRequest.objects.filter(from_member=other, to_member=request.user).delete()
    return redirect(request.POST.get("next") or reverse("status", kwargs={"username": request.user.username}))


@login_required
def status_view(request, username):
    member = get_object_or_404(User.objects.select_related("memberprofile"), username=username)
    own = member == request.user
    if request.method == "POST" and own:
        profile = request.user.memberprofile
        profile.town = request.POST.get("town", "").strip()
        profile.country = request.POST.get("country", "").strip()
        profile.motto1 = request.POST.get("motto1", "").strip()
        profile.motto2 = request.POST.get("motto2", "").strip()
        profile.likes = request.POST.get("likes", "").strip()
        profile.dislikes = request.POST.get("dislikes", "").strip()
        gps = request.POST.get("gps_coordinates", "").strip()
        if gps and not GPS_RE.match(gps):
            messages.error(request, "GPS coordinates must look like: 52.5N, 13.4E")
        else:
            profile.gps_coordinates = gps
            profile.enneagramtype1 = int(request.POST.get("enneagramtype1", 0) or 0)
            profile.enneagramtype2 = int(request.POST.get("enneagramtype2", 10) or 10)
            profile.save()
            member.first_name = request.POST.get("fullname", "").strip()
            member.email = request.POST.get("email", "").strip()
            member.save()
            messages.success(request, "Profile updated.")
            return redirect("status", username=member.username)

    relation = _status(request.user, member)
    show_contact = own or relation == "in_contact"
    ttt = TttResult.objects.filter(user=member).first()

    in_contact = _annotate_members(request.user, [u for u in User.objects.exclude(id=request.user.id) if _status(request.user, u) == "in_contact"])
    sent = _annotate_members(request.user, [u for u in User.objects.exclude(id=request.user.id) if _status(request.user, u) == "RCD_sent"])
    received = _annotate_members(request.user, [u for u in User.objects.exclude(id=request.user.id) if _status(request.user, u) == "RCD_received"])

    return render(
        request,
        "portal/status.html",
        {
            "member": member,
            "profile": member.memberprofile,
            "own": own,
            "show_contact": show_contact,
            "ttt": ttt,
            "relation": relation,
            "in_contact": in_contact,
            "sent": sent,
            "received": received,
        },
    )


@login_required
def member_plot(request):
    usernames = [u for u in request.GET.get("members", "").split(",") if u]
    x_var = request.GET.get("x", "ei")
    y_var = request.GET.get("y", "sn")
    users = list(User.objects.filter(username__in=usernames).select_related("memberprofile"))
    if request.user not in users:
        users.append(request.user)

    image = Image.new("RGB", (480, 360), "white")
    draw = ImageDraw.Draw(image)
    draw.rectangle((40, 20, 460, 320), outline="black", width=1)
    draw.line((40, 170, 460, 170), fill="#999")
    draw.line((250, 20, 250, 320), fill="#999")
    rng = random.Random(7)

    def var(result, key):
        if not result:
            return 0
        if key == "ei":
            return result.ei
        if key == "sn":
            return result.sn
        if key == "tf":
            return result.tf
        if key == "jp":
            return result.jp
        return 0

    colors = {"self": "blue", "no_contact": "black", "RCD_sent": "red", "in_contact": "green", "RCD_received": "purple"}
    for u in users:
        result = TttResult.objects.filter(user=u).first()
        x = var(result, x_var) + rng.uniform(-0.33, 0.33)
        y = var(result, y_var) + rng.uniform(-0.33, 0.33)
        px = int(250 + x * 12)
        py = int(170 - y * 12)
        status = _status(request.user, u)
        draw.ellipse((px - 4, py - 4, px + 4, py + 4), fill=colors[status])

    draw.text((200, 330), f"X: {x_var}  Y: {y_var}", fill="black")

    buf = BytesIO()
    image.save(buf, format="PNG")
    return HttpResponse(buf.getvalue(), content_type="image/png")
