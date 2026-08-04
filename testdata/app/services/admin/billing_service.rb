class BillingService < BaseService
  DEFAULT_CURRENCY = "USD"

  include Billable

  def call
    charge
  end

  private

  def charge
    true
  end
end