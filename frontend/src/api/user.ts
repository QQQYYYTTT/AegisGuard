import { http } from "@/utils/http";

export type UserSession = {
  avatar: string;
  username: string;
  nickname: string;
  roles: Array<string>;
  permissions: Array<string>;
  accessToken: string;
  refreshToken: string;
  expires: Date;
};

export type UserResult = {
  success: boolean;
  message?: string;
  data: UserSession;
};

export type RefreshTokenResult = {
  success: boolean;
  message?: string;
  data: {
    accessToken: string;
    refreshToken: string;
    expires: Date;
  };
};

export type RegisterPayload = {
  username: string;
  password: string;
  nickname?: string;
};

export type UserProfileResult = {
  success: boolean;
  message?: string;
  data: {
    id: number;
    username: string;
    nickname: string;
    created_at: string;
  };
};

export type LogoutResult = {
  success: boolean;
  message?: string;
};

export const getLogin = (data?: object) => {
  return http.request<UserResult>("post", "/api/user/login", { data });
};

export const registerApi = (data?: RegisterPayload) => {
  return http.request<UserResult>("post", "/api/user/register", { data });
};

export const refreshTokenApi = (data?: object) => {
  return http.request<RefreshTokenResult>("post", "/api/user/refresh", {
    data
  });
};

export const getProfileApi = () => {
  return http.request<UserProfileResult>("get", "/api/user/profile");
};

export const logoutApi = () => {
  return http.request<LogoutResult>("post", "/api/user/logout");
};
