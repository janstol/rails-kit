Rails.application.routes.draw do
  root to: "home#index"

  resources :users, only: [:index, :show]

  resources :posts do
    resources :comments, only: [:create, :destroy]
  end

  namespace :admin do
    resources :dashboards, only: [:show]
  end

  get "/pages/:slug", to: "pages#show"
  post "/login", to: "sessions#create"
  delete "/logout", to: "sessions#destroy"
end
