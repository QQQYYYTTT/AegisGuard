import { http } from "@/utils/http";

export type AssistantMessage = {
  role: "assistant" | "user";
  content: string;
};

type ApiResult<T> = {
  success: boolean;
  data?: T;
  message?: string;
};

export type AssistantChatResponse = {
  model: string;
  message: string;
};

export const chatWithAssistant = (data: {
  message: string;
  messages?: AssistantMessage[];
}) => {
  return http.request<ApiResult<AssistantChatResponse>>(
    "post",
    "/api/assistant/chat",
    { data },
    { timeout: 60000 }
  );
};
