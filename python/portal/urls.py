"""URL routes for the PbT portal."""

from django.urls import path

from . import views

urlpatterns = [
    path("", views.home, name="home"),
    path("register/", views.register_view, name="register"),
    path("login/", views.login_view, name="login"),
    path("logout/", views.logout_view, name="logout"),
    path("ttt/", views.ttt_view, name="ttt"),
    path("search/", views.search_view, name="search"),
    path("rcd/", views.send_rcd_view, name="send_rcd"),
    path("status/<str:username>/", views.status_view, name="status"),
    path("members/plot.png", views.member_plot, name="member_plot"),
]
